package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

// PostDna gestisce il "DNA" (il like dei pensieri) in modalità toggle: se il
// viewer ha già lasciato il proprio DNA lo rimuove, altrimenti lo aggiunge e
// notifica l'autore. Risponde con il frammento aggiornato del pulsante (HTMX).
func PostDna(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")

	var authorID string
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT author_id FROM pensieri WHERE id = $1`, pensieroID).Scan(&authorID); err != nil {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	// toggle: prova a rimuovere un DNA esistente; se non ce n'era, lo aggiunge.
	tag, err := db.Pool.Exec(context.Background(),
		`DELETE FROM dna_likes WHERE pensiero_id = $1 AND user_id = $2`, pensieroID, user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore")
		return
	}

	done := false
	if tag.RowsAffected() == 0 {
		if _, err := db.Pool.Exec(context.Background(), `
			INSERT INTO dna_likes (pensiero_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, pensieroID, user.ID); err != nil {
			c.String(http.StatusInternalServerError, "errore")
			return
		}
		done = true

		// notifica l'autore (mai per il DNA sul proprio pensiero)
		if authorID != user.ID {
			notify(authorID, "dna", map[string]string{
				"liker_name":  user.Username,
				"pensiero_id": pensieroID,
			})
		}
	}

	var count int
	db.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM dna_likes WHERE pensiero_id = $1`, pensieroID).Scan(&count)

	c.HTML(http.StatusOK, "dna-button", gin.H{
		"PensieroID": pensieroID,
		"DnaCount":   count,
		"DnaDone":    done,
	})
}
