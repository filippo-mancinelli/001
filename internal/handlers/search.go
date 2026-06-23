package handlers

import (
	"context"
	"net/http"
	"strings"

	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

func GetSearch(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	q := strings.TrimSpace(c.Query("q"))
	results, label := queryUsers(user, q)
	c.HTML(http.StatusOK, "search.html", gin.H{
		"User":    user,
		"Query":   q,
		"Results": results,
		"Label":   label,
	})
}

// GetSearchResults è l'endpoint HTMX che restituisce solo il fragment dei risultati.
func GetSearchResults(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	q := strings.TrimSpace(c.Query("q"))
	results, label := queryUsers(user, q)
	c.HTML(http.StatusOK, "search-results", gin.H{
		"User":    user,
		"Query":   q,
		"Results": results,
		"Label":   label,
	})
}

// queryUsers cerca utenti per username (q != "") oppure restituisce i suggeriti
// dal network (utenti non ancora seguiti, ordinati per numero di follower).
// Query con meno di 2 caratteri vengono trattate come vuote.
func queryUsers(user models.User, q string) ([]models.User, string) {
	var results []models.User

	if len(q) >= 2 {
		rows, err := db.Pool.Query(context.Background(), `
			SELECT id, username, COALESCE(bio, ''), COALESCE(display_name,''),
			       COALESCE(avatar_url,''), `+models.PresenceExpr("")+`
			FROM users
			WHERE username ILIKE '%' || $1 || '%' AND id <> $2
			ORDER BY username
			LIMIT 20
		`, q, user.ID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var u models.User
				rows.Scan(&u.ID, &u.Username, &u.Bio, &u.DisplayName, &u.AvatarURL, &u.Presence)
				results = append(results, u)
			}
		}
		return results, `risultati per "` + q + `"`
	}

	// Suggeriti: utenti con più follower che il viewer non segue ancora.
	rows, err := db.Pool.Query(context.Background(), `
		SELECT u.id, u.username, COALESCE(u.bio, ''), COALESCE(u.display_name,''),
		       COALESCE(u.avatar_url,''), `+models.PresenceExpr("u")+`
		FROM users u
		LEFT JOIN follows f ON f.follower_id = $1 AND f.following_id = u.id
		WHERE u.id <> $1 AND f.following_id IS NULL
		ORDER BY (SELECT COUNT(*) FROM follows WHERE following_id = u.id) DESC, u.created_at ASC
		LIMIT 10
	`, user.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u models.User
			rows.Scan(&u.ID, &u.Username, &u.Bio, &u.DisplayName, &u.AvatarURL, &u.Presence)
			results = append(results, u)
		}
	}
	return results, "suggeriti dal network"
}
