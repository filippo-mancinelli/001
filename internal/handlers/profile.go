package handlers

import (
	"context"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

func GetFeed(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT DISTINCT ON (t.id)
			t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			COALESCE(
				(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id = $1),
				(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id IS NULL)
			) AS content,
			(t.subject_id = $1) AS is_direct
		FROM thoughts t
		JOIN follows f ON f.following_id = t.author_id AND f.follower_id = $1
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		ORDER BY t.id, t.updated_at DESC
	`, user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "feed.html", gin.H{"User": user})
		return
	}
	defer rows.Close()

	var thoughts []models.ResolvedThought
	for rows.Next() {
		var rt models.ResolvedThought
		rows.Scan(&rt.ThoughtID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName, &rt.Content, &rt.IsDirect)
		if rt.Content != "" {
			thoughts = append(thoughts, rt)
		}
	}

	c.HTML(http.StatusOK, "feed.html", gin.H{"User": user, "Thoughts": thoughts})
}

func GetProfile(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	username := c.Param("username")

	var profile models.User
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id, username, email, bio, created_at FROM users WHERE username = $1`, username,
	).Scan(&profile.ID, &profile.Username, &profile.Email, &profile.Bio, &profile.CreatedAt)
	if err != nil {
		c.String(http.StatusNotFound, "utente non trovato")
		return
	}

	isSelf := profile.ID == user.ID

	var isFollowing bool
	if !isSelf {
		db.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id=$1 AND following_id=$2)`,
			user.ID, profile.ID,
		).Scan(&isFollowing)
	}

	// connessioni del profilo visualizzato
	followRows, _ := db.Pool.Query(context.Background(),
		`SELECT u.id, u.username FROM follows f JOIN users u ON u.id = f.following_id WHERE f.follower_id = $1`,
		profile.ID,
	)
	defer followRows.Close()
	var following []models.User
	for followRows.Next() {
		var u models.User
		followRows.Scan(&u.ID, &u.Username)
		following = append(following, u)
	}

	// pensieri scritti dal profilo, risolti per il viewer corrente
	thoughtRows, _ := db.Pool.Query(context.Background(), `
		SELECT t.id, t.author_id, u_a.username, t.subject_id, u_s.username,
			COALESCE(
				(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id = $2),
				(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id IS NULL)
			) AS content,
			(t.subject_id = $2 OR t.author_id = $2) AS is_direct
		FROM thoughts t
		JOIN users u_a ON u_a.id = t.author_id
		JOIN users u_s ON u_s.id = t.subject_id
		WHERE t.author_id = $1
		ORDER BY t.updated_at DESC
	`, profile.ID, user.ID)
	defer thoughtRows.Close()

	var thoughts []models.ResolvedThought
	for thoughtRows.Next() {
		var rt models.ResolvedThought
		thoughtRows.Scan(&rt.ThoughtID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName, &rt.Content, &rt.IsDirect)
		if rt.Content != "" {
			thoughts = append(thoughts, rt)
		}
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"User":        user,
		"Profile":     profile,
		"IsSelf":      isSelf,
		"IsFollowing": isFollowing,
		"Following":   following,
		"Thoughts":    thoughts,
	})
}
