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
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
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
			c.SetCookie("session_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
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
