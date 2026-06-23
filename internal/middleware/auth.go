package middleware

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			// nessuna sessione: è un visitatore non autenticato che ha provato
			// a usare una funzione riservata -> invito a registrarsi.
			redirectGuest(c, "/register")
			return
		}

		var user models.User
		row := db.Pool.QueryRow(context.Background(), `
			SELECT `+models.UserColumns("u")+`
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now())
		if err = models.ScanUser(row, &user); err != nil {
			// sessione scaduta o non valida: torna al login.
			c.SetCookie("session_token", "", -1, "/", "", false, true)
			redirectGuest(c, "/login")
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

// redirectGuest interrompe la richiesta indirizzando l'utente a dest. Per le
// richieste HTMX usa l'header HX-Redirect (HTMX naviga l'intera pagina),
// altrimenti un classico redirect 302.
func redirectGuest(c *gin.Context, dest string) {
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", dest)
		c.Status(http.StatusOK)
	} else {
		c.Redirect(http.StatusFound, dest)
	}
	c.Abort()
}

func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			c.Next()
			return
		}

		var user models.User
		row := db.Pool.QueryRow(context.Background(), `
			SELECT `+models.UserColumns("u")+`
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now())
		if err = models.ScanUser(row, &user); err == nil {
			c.Set("user", user)
		}
		c.Next()
	}
}
