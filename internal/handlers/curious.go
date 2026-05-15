package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

func PostCurious(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	thoughtID := c.Param("id")

	// recupera il subject del thought per inviargli la notifica
	var subjectID, authorID string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT subject_id, author_id FROM thoughts WHERE id = $1`, thoughtID,
	).Scan(&subjectID, &authorID)
	if err != nil {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	_, err = db.Pool.Exec(context.Background(), `
		INSERT INTO curious_requests (thought_id, requester_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, thoughtID, user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore")
		return
	}

	// recupera ID della curious_request appena creata
	var reqID string
	db.Pool.QueryRow(context.Background(),
		`SELECT id FROM curious_requests WHERE thought_id=$1 AND requester_id=$2`,
		thoughtID, user.ID,
	).Scan(&reqID)

	// notifica al subject
	payload, _ := json.Marshal(map[string]string{
		"requester_id":   user.ID,
		"requester_name": user.Username,
		"thought_id":     thoughtID,
		"request_id":     reqID,
	})
	db.Pool.Exec(context.Background(), `
		INSERT INTO notifications (user_id, type, payload) VALUES ($1, 'curious_request', $2)
	`, subjectID, string(payload))

	// risposta HTMX: mostra stato pending
	c.Data(http.StatusOK, "text/html", []byte(`
		<div class="thought-actions">
			<span style="color:var(--amber); font-size:0.75rem">richiesta inviata — in attesa...</span>
		</div>
	`))
}

func PostCuriousAccept(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	reqID := c.Param("id")

	var requesterID, thoughtID string
	err := db.Pool.QueryRow(context.Background(), `
		SELECT cr.requester_id, cr.thought_id
		FROM curious_requests cr
		JOIN thoughts t ON t.id = cr.thought_id
		WHERE cr.id = $1 AND t.subject_id = $2
	`, reqID, user.ID).Scan(&requesterID, &thoughtID)
	if err != nil {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	db.Pool.Exec(context.Background(),
		`UPDATE curious_requests SET status = 'accepted' WHERE id = $1`, reqID)

	// notifica al requester
	payload, _ := json.Marshal(map[string]string{
		"thought_id":   thoughtID,
		"accepted_by":  user.Username,
	})
	db.Pool.Exec(context.Background(), `
		INSERT INTO notifications (user_id, type, payload) VALUES ($1, 'curious_accepted', $2)
	`, requesterID, string(payload))

	c.Data(http.StatusOK, "text/html", []byte(`
		<div class="notif-row" style="color:var(--green); font-size:0.8rem">
			richiesta accettata — il richiedente potrà ora vedere il pensiero diretto.
		</div>
	`))
}

func PostCuriousReject(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	reqID := c.Param("id")

	var exists bool
	db.Pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM curious_requests cr
			JOIN thoughts t ON t.id = cr.thought_id
			WHERE cr.id = $1 AND t.subject_id = $2
		)
	`, reqID, user.ID).Scan(&exists)

	if !exists {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	db.Pool.Exec(context.Background(),
		`UPDATE curious_requests SET status = 'rejected' WHERE id = $1`, reqID)

	c.Data(http.StatusOK, "text/html", []byte(`
		<div class="notif-row" style="color:var(--text-dim); font-size:0.8rem">
			richiesta rifiutata.
		</div>
	`))
}
