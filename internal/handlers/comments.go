package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

const maxLunghezzaCommento = 1000
const commentPageSize = 20

// GetCommenti restituisce il thread dei commenti di un pensiero (lista + form),
// usato per espandere i commenti sotto la card via HTMX.
// Con ?offset=N restituisce solo i commenti aggiuntivi (paginazione "load more").
func GetCommenti(c *gin.Context) {
	user, auth := currentViewer(c)
	pensieroID := c.Param("id")

	if !pensieroEsiste(pensieroID) {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}

	if offset > 0 {
		renderCommentiPiu(c, pensieroID, user, offset)
	} else {
		renderThreadCommenti(c, pensieroID, user, !auth)
	}
}

// GetCommentiChiudi restituisce solo il pulsante toggle iniziale,
// permettendo di collassare i commenti tramite HTMX.
func GetCommentiChiudi(c *gin.Context) {
	pensieroID := c.Param("id")
	var count int
	db.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM commenti WHERE pensiero_id = $1`, pensieroID).Scan(&count)
	c.HTML(http.StatusOK, "commenti-chiudi", gin.H{
		"PensieroID":   pensieroID,
		"CommentCount": count,
	})
}

// GetUserMini restituisce la mini card utente per i tooltip sullo username.
func GetUserMini(c *gin.Context) {
	username := c.Param("username")
	var u models.User
	err := db.Pool.QueryRow(context.Background(),
		`SELECT username, COALESCE(display_name, '') FROM users WHERE username = $1`,
		username).Scan(&u.Username, &u.DisplayName)
	if err != nil {
		u.Username = username
	}
	c.HTML(http.StatusOK, "user-mini-card", u)
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

	renderThreadCommenti(c, pensieroID, user, false)
}

// DeleteCommento elimina un commento. È consentito all'autore del commento,
// all'autore del pensiero o a un amministratore (moderazione di contenuti
// sensibili o vietati). Ri-renderizza il thread aggiornato.
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

	if user.ID != commentAuthorID && user.ID != pensieroAuthorID && !user.IsAdmin {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	db.Pool.Exec(context.Background(), `DELETE FROM commenti WHERE id = $1`, commentoID)

	renderThreadCommenti(c, pensieroID, user, false)
}

// pensieroEsiste verifica la presenza di un pensiero per id.
func pensieroEsiste(pensieroID string) bool {
	var exists bool
	db.Pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pensieri WHERE id = $1)`, pensieroID).Scan(&exists)
	return exists
}

// renderThreadCommenti carica i primi commentPageSize commenti e rende il
// partial "commenti-thread" con eventuale pulsante "carica altri".
func renderThreadCommenti(c *gin.Context, pensieroID string, viewer models.User, anon bool) {
	commenti := queryCommenti(pensieroID, viewer, 0)

	hasMore := len(commenti) > commentPageSize
	if hasMore {
		commenti = commenti[:commentPageSize]
	}

	c.HTML(http.StatusOK, "commenti-thread", gin.H{
		"PensieroID": pensieroID,
		"Commenti":   commenti,
		"HasMore":    hasMore,
		"NextOffset": commentPageSize,
		"Anon":       anon,
	})
}

// renderCommentiPiu carica una pagina successiva di commenti e rende
// il partial "commenti-piu" (solo i nuovi item + eventuale nuovo bottone).
func renderCommentiPiu(c *gin.Context, pensieroID string, viewer models.User, offset int) {
	commenti := queryCommenti(pensieroID, viewer, offset)

	hasMore := len(commenti) > commentPageSize
	if hasMore {
		commenti = commenti[:commentPageSize]
	}

	c.HTML(http.StatusOK, "commenti-piu", gin.H{
		"PensieroID": pensieroID,
		"Commenti":   commenti,
		"HasMore":    hasMore,
		"NextOffset": offset + commentPageSize,
	})
}

func queryCommenti(pensieroID string, viewer models.User, offset int) []models.Commento {
	rows, err := db.Pool.Query(context.Background(), `
		SELECT cm.id, cm.author_id, u.username, COALESCE(u.avatar_url, ''), cm.content, cm.created_at,
		       (cm.author_id = $2 OR p.author_id = $2 OR $5) AS can_delete
		FROM commenti cm
		JOIN users u ON u.id = cm.author_id
		JOIN pensieri p ON p.id = cm.pensiero_id
		WHERE cm.pensiero_id = $1
		ORDER BY cm.created_at ASC
		LIMIT $3 OFFSET $4
	`, pensieroID, viewer.ID, commentPageSize+1, offset, viewer.IsAdmin)

	var commenti []models.Commento
	if err != nil {
		return commenti
	}
	defer rows.Close()
	for rows.Next() {
		var cm models.Commento
		cm.PensieroID = pensieroID
		if rows.Scan(&cm.ID, &cm.AuthorID, &cm.AuthorName, &cm.AuthorAvatar, &cm.Content,
			&cm.CreatedAt, &cm.CanDelete) == nil {
			commenti = append(commenti, cm)
		}
	}
	return commenti
}
