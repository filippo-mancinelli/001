package middleware

import (
	"context"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"
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
		err = db.Pool.QueryRow(context.Background(), `
			SELECT u.id, u.username, u.email, COALESCE(u.bio, ''), u.created_at
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now()).Scan(
			&user.ID, &user.Username, &user.Email, &user.Bio, &user.CreatedAt,
		)
		if err != nil {
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
		err = db.Pool.QueryRow(context.Background(), `
			SELECT u.id, u.username, u.email, COALESCE(u.bio, ''), u.created_at
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now()).Scan(
			&user.ID, &user.Username, &user.Email, &user.Bio, &user.CreatedAt,
		)
		if err == nil {
			c.Set("user", user)
		}
		c.Next()
	}
}
