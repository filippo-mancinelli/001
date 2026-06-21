package handlers

import (
	"context"
	"net/http"
	"strings"

	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

// maxLunghezzaCommento limita la dimensione di un commento per evitare abusi.
const maxLunghezzaCommento = 1000

// GetCommenti restituisce il thread dei commenti di un pensiero (lista + form),
// usato per espandere i commenti sotto la card via HTMX.
func GetCommenti(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")

	if !pensieroEsiste(pensieroID) {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	renderThreadCommenti(c, pensieroID, user.ID)
}

// PostCommento aggiunge un commento a un pensiero e ri-renderizza il thread.
// Notifica l'autore del pensiero (se diverso da chi commenta).
func PostCommento(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")
	content := strings.TrimSpace(c.PostForm("content"))

	if content == "" {
		c.String(http.StatusBadRequest, "commento vuoto")
		return
	}
	if len([]rune(content)) > maxLunghezzaCommento {
		content = string([]rune(content)[:maxLunghezzaCommento])
	}

	// recupera autore e subject del pensiero (anche per validarne l'esistenza)
	var authorID, subjectID string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT author_id, subject_id FROM pensieri WHERE id = $1`, pensieroID,
	).Scan(&authorID, &subjectID)
	if err != nil {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	if _, err := db.Pool.Exec(context.Background(), `
		INSERT INTO commenti (pensiero_id, author_id, content)
		VALUES ($1, $2, $3)
	`, pensieroID, user.ID, content); err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio commento")
		return
	}

	// notifica l'autore del pensiero, se non sta commentando sé stesso
	if authorID != user.ID {
		notify(authorID, "new_comment", map[string]string{
			"commenter_id":   user.ID,
			"commenter_name": user.Username,
			"pensiero_id":    pensieroID,
		})
	}

	renderThreadCommenti(c, pensieroID, user.ID)
}

// DeleteCommento elimina un commento. È consentito all'autore del commento o
// all'autore del pensiero (moderazione). Ri-renderizza il thread aggiornato.
func DeleteCommento(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	commentoID := c.Param("id")

	var pensieroID, commentAuthorID, pensieroAuthorID string
	err := db.Pool.QueryRow(context.Background(), `
		SELECT cm.pensiero_id, cm.author_id, p.author_id
		FROM commenti cm
		JOIN pensieri p ON p.id = cm.pensiero_id
		WHERE cm.id = $1
	`, commentoID).Scan(&pensieroID, &commentAuthorID, &pensieroAuthorID)
	if err != nil {
		c.String(http.StatusNotFound, "commento non trovato")
		return
	}

	if user.ID != commentAuthorID && user.ID != pensieroAuthorID {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	db.Pool.Exec(context.Background(), `DELETE FROM commenti WHERE id = $1`, commentoID)

	renderThreadCommenti(c, pensieroID, user.ID)
}

// pensieroEsiste verifica la presenza di un pensiero per id.
func pensieroEsiste(pensieroID string) bool {
	var exists bool
	db.Pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pensieri WHERE id = $1)`, pensieroID).Scan(&exists)
	return exists
}

// renderThreadCommenti carica i commenti di un pensiero risolti per il viewer
// e rende il partial "commenti-thread".
func renderThreadCommenti(c *gin.Context, pensieroID, viewerID string) {
	rows, err := db.Pool.Query(context.Background(), `
		SELECT cm.id, cm.author_id, u.username, cm.content, cm.created_at,
		       (cm.author_id = $2 OR p.author_id = $2) AS can_delete
		FROM commenti cm
		JOIN users u ON u.id = cm.author_id
		JOIN pensieri p ON p.id = cm.pensiero_id
		WHERE cm.pensiero_id = $1
		ORDER BY cm.created_at ASC
	`, pensieroID, viewerID)

	var commenti []models.Commento
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cm models.Commento
			cm.PensieroID = pensieroID
			if rows.Scan(&cm.ID, &cm.AuthorID, &cm.AuthorName, &cm.Content,
				&cm.CreatedAt, &cm.CanDelete) == nil {
				commenti = append(commenti, cm)
			}
		}
	}

	c.HTML(http.StatusOK, "commenti-thread", gin.H{
		"PensieroID": pensieroID,
		"Commenti":   commenti,
	})
}
