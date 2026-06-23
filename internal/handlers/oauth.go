package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"pensieri/internal/db"
	mailer "pensieri/internal/email"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Configurazione Google OAuth (variabili d'ambiente):
//
//	GOOGLE_CLIENT_ID      client ID OAuth 2.0 della console Google Cloud.
//	GOOGLE_CLIENT_SECRET  client secret associato.
//	APP_URL               URL base dell'app (default http://localhost:8080).
//	                      Il redirect URI è APP_URL + "/auth/google/callback"
//	                      e deve essere autorizzato nella console Google.
//
// Se GOOGLE_CLIENT_ID o GOOGLE_CLIENT_SECRET non sono impostati, il pulsante
// Google nelle pagine di login/registrazione non viene mostrato e gli endpoint
// rispondono con un redirect a /login.

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserinfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleEnabled indica se l'OAuth Google è configurato. Usato dai template per
// mostrare o nascondere il pulsante.
func GoogleEnabled() bool {
	return os.Getenv("GOOGLE_CLIENT_ID") != "" && os.Getenv("GOOGLE_CLIENT_SECRET") != ""
}

func appURL() string {
	if v := os.Getenv("APP_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8080"
}

func googleRedirectURI() string {
	return appURL() + "/auth/google/callback"
}

// GetGoogleLogin avvia il flusso OAuth: genera uno state anti-CSRF, lo salva in
// un cookie e reindirizza l'utente alla pagina di consenso Google.
func GetGoogleLogin(c *gin.Context) {
	if !GoogleEnabled() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"error": "errore interno", "googleEnabled": GoogleEnabled()})
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	// Cookie state di breve durata (10 min), HttpOnly.
	c.SetCookie("oauth_state", state, 600, "/", "", false, true)

	q := url.Values{}
	q.Set("client_id", os.Getenv("GOOGLE_CLIENT_ID"))
	q.Set("redirect_uri", googleRedirectURI())
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")

	c.Redirect(http.StatusFound, googleAuthURL+"?"+q.Encode())
}

type googleUserinfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GetGoogleCallback gestisce il ritorno da Google: verifica lo state, scambia il
// code con un access token, recupera il profilo, trova o crea l'utente e apre
// una sessione.
func GetGoogleCallback(c *gin.Context) {
	if !GoogleEnabled() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// L'utente può aver negato il consenso.
	if errParam := c.Query("error"); errParam != "" {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "accesso con Google annullato", "googleEnabled": true})
		return
	}

	// Verifica state anti-CSRF.
	state := c.Query("state")
	stateCookie, _ := c.Cookie("oauth_state")
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)
	if state == "" || stateCookie == "" || state != stateCookie {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{"error": "sessione OAuth non valida, riprova", "googleEnabled": true})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{"error": "codice OAuth mancante", "googleEnabled": true})
		return
	}

	info, err := exchangeGoogleCode(code)
	if err != nil {
		log.Printf("google oauth: %v", err)
		c.HTML(http.StatusBadGateway, "login.html", gin.H{"error": "autenticazione Google fallita", "googleEnabled": true})
		return
	}
	if info.Email == "" {
		c.HTML(http.StatusBadGateway, "login.html", gin.H{"error": "Google non ha restituito un'email", "googleEnabled": true})
		return
	}

	userID, isNew, err := findOrCreateGoogleUser(info)
	if err != nil {
		log.Printf("google oauth user: %v", err)
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"error": "errore interno", "googleEnabled": true})
		return
	}

	if err := startSession(c, userID); err != nil {
		log.Printf("google oauth session: %v", err)
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"error": "errore interno", "googleEnabled": true})
		return
	}

	if isNew {
		var username string
		_ = db.Pool.QueryRow(context.Background(),
			`SELECT username FROM users WHERE id = $1`, userID).Scan(&username)
		subject, html := mailer.Welcome(username)
		mailer.SendAsync(info.Email, subject, html)
	}

	c.Redirect(http.StatusFound, "/")
}

// exchangeGoogleCode scambia il code di autorizzazione con un access token e
// recupera le informazioni del profilo.
func exchangeGoogleCode(code string) (*googleUserinfo, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", os.Getenv("GOOGLE_CLIENT_ID"))
	form.Set("client_secret", os.Getenv("GOOGLE_CLIENT_SECRET"))
	form.Set("redirect_uri", googleRedirectURI())
	form.Set("grant_type", "authorization_code")

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.PostForm(googleTokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<12))
		return nil, fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, body)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}

	req, err := http.NewRequest(http.MethodGet, googleUserinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	uresp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer uresp.Body.Close()
	if uresp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(uresp.Body, 1<<12))
		return nil, fmt.Errorf("userinfo status %d: %s", uresp.StatusCode, body)
	}

	var info googleUserinfo
	if err := json.NewDecoder(uresp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	return &info, nil
}

// findOrCreateGoogleUser collega l'identità Google a un utente esistente
// (tramite google_id o email) oppure ne crea uno nuovo. Ritorna l'id utente e
// se l'account è stato appena creato.
func findOrCreateGoogleUser(info *googleUserinfo) (id string, isNew bool, err error) {
	ctx := context.Background()

	// 1) Già collegato via google_id.
	err = db.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE google_id = $1`, info.ID).Scan(&id)
	if err == nil {
		return id, false, nil
	}

	// 2) Esiste un account con la stessa email: lo colleghiamo a Google.
	err = db.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1`, info.Email).Scan(&id)
	if err == nil {
		_, err = db.Pool.Exec(ctx,
			`UPDATE users SET google_id = $1 WHERE id = $2`, info.ID, id)
		if err != nil {
			return "", false, fmt.Errorf("link google_id: %w", err)
		}
		return id, false, nil
	}

	// 3) Nuovo utente: username derivato dall'email, reso univoco.
	username, err := uniqueUsername(ctx, info.Email)
	if err != nil {
		return "", false, err
	}

	display := strings.TrimSpace(info.Name)

	err = db.Pool.QueryRow(ctx,
		`INSERT INTO users (username, email, google_id, display_name)
		 VALUES ($1, $2, $3, NULLIF($4, '')) RETURNING id`,
		username, info.Email, info.ID, display,
	).Scan(&id)
	if err != nil {
		return "", false, fmt.Errorf("insert user: %w", err)
	}
	return id, true, nil
}

// uniqueUsername genera uno username valido (3-32 caratteri, [a-z0-9_]) a
// partire dall'email e garantisce l'unicità aggiungendo un suffisso numerico.
func uniqueUsername(ctx context.Context, email string) (string, error) {
	base := email
	if i := strings.IndexByte(base, '@'); i > 0 {
		base = base[:i]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	base = strings.Trim(b.String(), "_")
	if len(base) < 3 {
		base = "user"
	}
	if len(base) > 28 {
		base = base[:28]
	}

	candidate := base
	for i := 0; i < 1000; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i)
		}
		var exists bool
		if err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, candidate,
		).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	// Fallback estremamente improbabile.
	return base + uuid.NewString()[:8], nil
}

// startSession crea una sessione per l'utente e imposta il cookie.
func startSession(c *gin.Context, userID string) error {
	token := uuid.NewString()
	expires := time.Now().Add(30 * 24 * time.Hour)
	if _, err := db.Pool.Exec(context.Background(),
		`INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, expires,
	); err != nil {
		return err
	}
	c.SetCookie("session_token", token, int(30*24*time.Hour/time.Second), "/", "", false, true)
	return nil
}
