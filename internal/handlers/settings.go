package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var allowedPresence = map[string]bool{
	"online": true, "away": true, "busy": true, "offline": true,
}

// GetSettings mostra la pagina impostazioni con i valori correnti.
// I messaggi di esito vengono passati via query param (?saved=... / ?err=...).
func GetSettings(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"User":    user,
		"Profile": user,
		"Saved":   c.Query("saved"),
		"Err":     c.Query("err"),
	})
}

// PostProfileSettings aggiorna i campi di personalizzazione del profilo.
func PostProfileSettings(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	displayName := truncate(strings.TrimSpace(c.PostForm("display_name")), 64)
	status := truncate(strings.TrimSpace(c.PostForm("status")), 140)
	bio := truncate(strings.TrimSpace(c.PostForm("bio")), 1000)
	avatarURL := strings.TrimSpace(c.PostForm("avatar_url"))
	location := truncate(strings.TrimSpace(c.PostForm("location")), 120)
	website := truncate(strings.TrimSpace(c.PostForm("website")), 255)
	presence := strings.TrimSpace(c.PostForm("presence"))

	if !allowedPresence[presence] {
		presence = "online"
	}
	if avatarURL != "" && !strings.HasPrefix(avatarURL, "http://") && !strings.HasPrefix(avatarURL, "https://") {
		redirectSettings(c, "", "url-immagine-non-valido")
		return
	}

	_, err := db.Pool.Exec(context.Background(), `
		UPDATE users SET
			display_name = NULLIF($1, ''),
			status       = NULLIF($2, ''),
			bio          = NULLIF($3, ''),
			avatar_url   = NULLIF($4, ''),
			location     = NULLIF($5, ''),
			website      = NULLIF($6, ''),
			presence     = $7
		WHERE id = $8
	`, displayName, status, bio, avatarURL, location, website, presence, user.ID)
	if err != nil {
		redirectSettings(c, "", "errore-salvataggio")
		return
	}
	redirectSettings(c, "profilo", "")
}

// PostAccountSettings aggiorna username ed email (impostazioni generali
// dell'account). Lo username fa parte dell'identità dell'utente: è modificabile,
// purché valido e non già in uso da un altro account.
func PostAccountSettings(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))

	if !validUsername(username) {
		redirectSettings(c, "", "username-non-valido")
		return
	}
	if !strings.Contains(email, "@") || len(email) > 255 {
		redirectSettings(c, "", "email-non-valida")
		return
	}

	// Se lo username cambia, verifica che non sia già preso (confronto
	// case-insensitive per evitare collisioni/impersonificazioni come
	// "mario" vs "Mario"). Si ignora il proprio account.
	if !strings.EqualFold(username, user.Username) {
		var exists bool
		if err := db.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM users WHERE lower(username) = lower($1) AND id <> $2)`,
			username, user.ID).Scan(&exists); err != nil {
			redirectSettings(c, "", "errore-interno")
			return
		}
		if exists {
			redirectSettings(c, "", "username-gia-in-uso")
			return
		}
	}

	_, err := db.Pool.Exec(context.Background(),
		`UPDATE users SET username = $1, email = $2 WHERE id = $3`, username, email, user.ID)
	if err != nil {
		redirectSettings(c, "", "username-o-email-gia-in-uso")
		return
	}
	redirectSettings(c, "account", "")
}

// validUsername impone le stesse regole dello username auto-generato:
// 3-32 caratteri tra lettere, cifre e underscore.
func validUsername(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

// PostPasswordSettings cambia la password verificando quella attuale.
func PostPasswordSettings(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	current := c.PostForm("current_password")
	next := c.PostForm("new_password")

	var hash string
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT password_hash FROM users WHERE id = $1`, user.ID).Scan(&hash); err != nil {
		redirectSettings(c, "", "errore-interno")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		redirectSettings(c, "", "password-attuale-errata")
		return
	}
	if len(next) < 6 {
		redirectSettings(c, "", "password-troppo-corta")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		redirectSettings(c, "", "errore-interno")
		return
	}
	if _, err := db.Pool.Exec(context.Background(),
		`UPDATE users SET password_hash = $1 WHERE id = $2`, string(newHash), user.ID); err != nil {
		redirectSettings(c, "", "errore-salvataggio")
		return
	}

	// invalida le altre sessioni tranne quella corrente per sicurezza
	token, _ := c.Cookie("session_token")
	db.Pool.Exec(context.Background(),
		`DELETE FROM sessions WHERE user_id = $1 AND token <> $2`, user.ID, token)

	redirectSettings(c, "password", "")
}

func redirectSettings(c *gin.Context, saved, errMsg string) {
	q := url.Values{}
	if saved != "" {
		q.Set("saved", saved)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	target := "/settings"
	if e := q.Encode(); e != "" {
		target += "?" + e
	}
	c.Redirect(http.StatusFound, target)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}
