package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"pensieri/internal/db"
	"pensieri/internal/models"
	"pensieri/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var allowedPresence = map[string]bool{
	"online": true, "away": true, "busy": true, "offline": true,
}

// avatarExt mappa i content-type immagine ammessi per l'avatar all'estensione
// del file. Limita gli upload ai formati immagine sicuri e ampiamente supportati.
var avatarExt = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// maxAvatarBytes limita la dimensione dell'immagine avatar caricabile (5 MB).
const maxAvatarBytes = 5 << 20

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

	// Se è stato caricato un file immagine, ha la precedenza sul campo URL:
	// viene caricato su S3 e l'avatar punta al proxy /media/avatars/<file>.
	if uploaded, errMsg := uploadAvatarIfPresent(c, user.ID); errMsg != "" {
		redirectSettings(c, "", errMsg)
		return
	} else if uploaded != "" {
		avatarURL = uploaded
	}

	if avatarURL != "" && !strings.HasPrefix(avatarURL, "http://") &&
		!strings.HasPrefix(avatarURL, "https://") && !strings.HasPrefix(avatarURL, "/media/") {
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

// uploadAvatarIfPresent gestisce l'eventuale file "avatar_file" del form.
// Ritorna ("", "") se nessun file è stato caricato; in caso di upload riuscito
// ritorna l'URL relativo da salvare in avatar_url (es. "/media/avatars/<id>.jpg").
// Il secondo valore, se non vuoto, è un codice di errore da mostrare all'utente.
func uploadAvatarIfPresent(c *gin.Context, userID string) (string, string) {
	fh, err := c.FormFile("avatar_file")
	if err != nil || fh == nil {
		return "", "" // nessun file caricato
	}
	if !storage.Configured() {
		return "", "upload-non-disponibile"
	}
	if fh.Size > maxAvatarBytes {
		return "", "immagine-troppo-grande"
	}

	f, err := fh.Open()
	if err != nil {
		return "", "errore-upload"
	}
	defer f.Close()

	// Rileva il content-type dai primi byte del file (non ci si fida del nome
	// né dell'header inviato dal client), poi verifica che sia un'immagine ammessa.
	head := make([]byte, 512)
	n, _ := f.Read(head)
	contentType := http.DetectContentType(head[:n])
	ext, ok := avatarExt[contentType]
	if !ok {
		return "", "formato-immagine-non-valido"
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", "errore-upload"
	}

	name := fmt.Sprintf("%s-%s.%s", userID, uuid.NewString(), ext)
	if err := storage.Upload(c.Request.Context(), "avatars/"+name, contentType, f); err != nil {
		return "", "errore-upload"
	}
	return "/media/avatars/" + name, ""
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}
