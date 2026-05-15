package handlers

import (
	"context"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

func GetThoughtEditor(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, _ := db.Pool.Query(context.Background(),
		`SELECT u.id, u.username FROM follows f JOIN users u ON u.id = f.following_id WHERE f.follower_id = $1`,
		user.ID,
	)
	defer rows.Close()

	var following []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Username)
		following = append(following, u)
	}

	c.HTML(http.StatusOK, "thought-editor", gin.H{"Following": following})
}

func PostThought(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	subjectID := c.PostForm("subject_id")
	audienceID := c.PostForm("audience_id")
	content := c.PostForm("content")

	if subjectID == "" || content == "" {
		c.String(http.StatusBadRequest, "dati mancanti")
		return
	}

	// upsert thought meta
	var thoughtID string
	err := db.Pool.QueryRow(context.Background(), `
		INSERT INTO thoughts (author_id, subject_id)
		VALUES ($1, $2)
		ON CONFLICT (author_id, subject_id) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`, user.ID, subjectID).Scan(&thoughtID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio")
		return
	}

	// upsert versione
	var audID *string
	if audienceID != "" {
		audID = &audienceID
	}
	_, err = db.Pool.Exec(context.Background(), `
		INSERT INTO thought_versions (thought_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (thought_id, audience_id) DO UPDATE SET content = $3
	`, thoughtID, audID, content)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio versione")
		return
	}

	// risposta HTMX: svuota editor con messaggio conferma
	c.Data(http.StatusOK, "text/html", []byte(`
		<div style="color:var(--green); font-size:0.8rem; padding:0.5rem 0">
			[ pensiero salvato ] — <a href="">ricarica la pagina</a>
		</div>
	`))
}

func PostThoughtVersion(c *gin.Context) {
	thoughtID := c.Param("id")
	audienceID := c.PostForm("audience_id")
	content := c.PostForm("content")

	var audID *string
	if audienceID != "" {
		audID = &audienceID
	}

	db.Pool.Exec(context.Background(), `
		INSERT INTO thought_versions (thought_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (thought_id, audience_id) DO UPDATE SET content = $3
	`, thoughtID, audID, content)

	c.Status(http.StatusNoContent)
}

func DeleteThoughtVersion(c *gin.Context) {
	thoughtID := c.Param("id")
	audienceID := c.Param("audienceID")

	if audienceID == "default" {
		db.Pool.Exec(context.Background(),
			`DELETE FROM thought_versions WHERE thought_id = $1 AND audience_id IS NULL`, thoughtID)
	} else {
		db.Pool.Exec(context.Background(),
			`DELETE FROM thought_versions WHERE thought_id = $1 AND audience_id = $2`, thoughtID, audienceID)
	}

	c.Status(http.StatusNoContent)
}
