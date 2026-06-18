package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"thoughts/internal/db"
	"thoughts/internal/email"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// resetTokenTTL definisce la durata di validità di un link di reset.
const resetTokenTTL = time.Hour

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetForgot mostra il form "password dimenticata".
func GetForgot(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot.html", gin.H{})
}

// PostForgot genera un token di reset e invia l'email.
// Risponde sempre con lo stesso messaggio per non rivelare se l'email esiste.
func PostForgot(c *gin.Context) {
	addr := strings.TrimSpace(c.PostForm("email"))

	var userID, username string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, username FROM users WHERE email = $1`, addr,
	).Scan(&userID, &username)

	if err == nil {
		token := newToken()
		_, dberr := db.Pool.Exec(context.Background(), `
			INSERT INTO password_resets (user_id, token, expires_at)
			VALUES ($1, $2, $3)
		`, userID, token, time.Now().Add(resetTokenTTL))
		if dberr == nil {
			subject, html := email.PasswordReset(username, token)
			email.SendAsync(addr, subject, html)
		}
	}

	c.HTML(http.StatusOK, "forgot.html", gin.H{
		"sent": true,
	})
}

// lookupReset valida un token restituendo l'id utente associato.
func lookupReset(token string) (userID string, ok bool) {
	if token == "" {
		return "", false
	}
	err := db.Pool.QueryRow(context.Background(), `
		SELECT user_id FROM password_resets
		WHERE token = $1 AND used = FALSE AND expires_at > $2
	`, token, time.Now()).Scan(&userID)
	return userID, err == nil
}

// GetReset mostra il form per impostare la nuova password.
func GetReset(c *gin.Context) {
	token := c.Query("token")
	if _, ok := lookupReset(token); !ok {
		c.HTML(http.StatusOK, "reset.html", gin.H{"invalid": true})
		return
	}
	c.HTML(http.StatusOK, "reset.html", gin.H{"token": token})
}

// PostReset verifica il token, aggiorna la password e invalida le sessioni.
func PostReset(c *gin.Context) {
	token := c.PostForm("token")
	password := c.PostForm("password")

	userID, ok := lookupReset(token)
	if !ok {
		c.HTML(http.StatusOK, "reset.html", gin.H{"invalid": true})
		return
	}
	if len(password) < 6 {
		c.HTML(http.StatusOK, "reset.html", gin.H{
			"token": token,
			"error": "la password deve avere almeno 6 caratteri",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusOK, "reset.html", gin.H{"token": token, "error": "errore interno"})
		return
	}

	if _, err := db.Pool.Exec(context.Background(),
		`UPDATE users SET password_hash = $1 WHERE id = $2`, string(hash), userID); err != nil {
		c.HTML(http.StatusOK, "reset.html", gin.H{"token": token, "error": "errore salvataggio"})
		return
	}

	// invalida il token (uso singolo) e tutte le sessioni esistenti
	db.Pool.Exec(context.Background(),
		`UPDATE password_resets SET used = TRUE WHERE token = $1`, token)
	db.Pool.Exec(context.Background(),
		`DELETE FROM sessions WHERE user_id = $1`, userID)

	c.HTML(http.StatusOK, "reset.html", gin.H{"done": true})
}
