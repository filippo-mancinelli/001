package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

func GetEditorPensiero(c *gin.Context) {
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

	c.HTML(http.StatusOK, "pensiero-editor", gin.H{"Following": following})
}

// AnnullaEditorPensiero svuota l'area dell'editor (risposta HTMX).
func AnnullaEditorPensiero(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(""))
}

func PostPensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	subjectID := c.PostForm("subject_id")
	audienceID := c.PostForm("audience_id")
	content := c.PostForm("content")

	if subjectID == "" || content == "" {
		c.String(http.StatusBadRequest, "dati mancanti")
		return
	}

	// upsert pensiero meta
	var pensieroID string
	err := db.Pool.QueryRow(context.Background(), `
		INSERT INTO pensieri (author_id, subject_id)
		VALUES ($1, $2)
		ON CONFLICT (author_id, subject_id) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`, user.ID, subjectID).Scan(&pensieroID)
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
		INSERT INTO versioni_pensiero (pensiero_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (pensiero_id, audience_id) DO UPDATE SET content = $3
	`, pensieroID, audID, content)
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

func PostVersionePensiero(c *gin.Context) {
	pensieroID := c.Param("id")
	audienceID := c.PostForm("audience_id")
	content := c.PostForm("content")

	var audID *string
	if audienceID != "" {
		audID = &audienceID
	}

	db.Pool.Exec(context.Background(), `
		INSERT INTO versioni_pensiero (pensiero_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (pensiero_id, audience_id) DO UPDATE SET content = $3
	`, pensieroID, audID, content)

	c.Status(http.StatusNoContent)
}

func EliminaVersionePensiero(c *gin.Context) {
	pensieroID := c.Param("id")
	audienceID := c.Param("audienceID")

	if audienceID == "default" {
		db.Pool.Exec(context.Background(),
			`DELETE FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id IS NULL`, pensieroID)
	} else {
		db.Pool.Exec(context.Background(),
			`DELETE FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id = $2`, pensieroID, audienceID)
	}

	c.Status(http.StatusNoContent)
}
