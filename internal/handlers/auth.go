package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	mailer "pensieri/internal/email"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func GetLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{"googleEnabled": GoogleEnabled()})
}

func PostLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var id, hash string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, COALESCE(password_hash, '') FROM users WHERE username = $1`, username,
	).Scan(&id, &hash)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "credenziali non valide", "googleEnabled": GoogleEnabled()})
		return
	}

	// Account creato solo con Google: nessuna password locale.
	if hash == "" {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "questo account usa l'accesso con Google", "googleEnabled": GoogleEnabled()})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "credenziali non valide", "googleEnabled": GoogleEnabled()})
		return
	}

	token := uuid.NewString()
	expires := time.Now().Add(30 * 24 * time.Hour)
	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		id, token, expires,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"error": "errore interno"})
		return
	}

	c.SetCookie("session_token", token, int(30*24*time.Hour/time.Second), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func GetRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{"googleEnabled": GoogleEnabled()})
}

func PostRegister(c *gin.Context) {
	username := c.PostForm("username")
	email := c.PostForm("email")
	password := c.PostForm("password")

	if len(username) < 3 || len(password) < 6 {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"error": "username min 3 caratteri, password min 6",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{"error": "errore interno"})
		return
	}

	var id string
	err = db.Pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		username, email, string(hash),
	).Scan(&id)
	if err != nil {
		c.HTML(http.StatusConflict, "register.html", gin.H{"error": "username o email già in uso"})
		return
	}

	// email di benvenuto (asincrona, non blocca la registrazione)
	subject, html := mailer.Welcome(username)
	mailer.SendAsync(email, subject, html)

	token := uuid.NewString()
	expires := time.Now().Add(30 * 24 * time.Hour)
	db.Pool.Exec(context.Background(),
		`INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		id, token, expires,
	)

	c.SetCookie("session_token", token, int(30*24*time.Hour/time.Second), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func PostLogout(c *gin.Context) {
	token, _ := c.Cookie("session_token")
	if token != "" {
		db.Pool.Exec(context.Background(), `DELETE FROM sessions WHERE token = $1`, token)
	}
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
