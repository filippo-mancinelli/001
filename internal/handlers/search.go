package handlers

import (
	"context"
	"net/http"
	"strings"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

// GetSearch cerca utenti per username (match parziale, case-insensitive) ed
// esclude il viewer corrente dai risultati.
func GetSearch(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	q := strings.TrimSpace(c.Query("q"))

	var results []models.User
	if q != "" {
		rows, err := db.Pool.Query(context.Background(), `
			SELECT id, username, COALESCE(bio, ''), COALESCE(display_name,''), COALESCE(avatar_url,''), COALESCE(presence,'online')
			FROM users
			WHERE username ILIKE '%' || $1 || '%' AND id <> $2
			ORDER BY username
			LIMIT 50
		`, q, user.ID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var u models.User
				rows.Scan(&u.ID, &u.Username, &u.Bio, &u.DisplayName, &u.AvatarURL, &u.Presence)
				results = append(results, u)
			}
		}
	}

	c.HTML(http.StatusOK, "search.html", gin.H{
		"User":    user,
		"Query":   q,
		"Results": results,
	})
}
