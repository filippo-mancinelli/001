package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

func PostCurious(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")

	// recupera il subject del pensiero per inviargli la notifica
	var subjectID, authorID string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT subject_id, author_id FROM pensieri WHERE id = $1`, pensieroID,
	).Scan(&subjectID, &authorID)
	if err != nil {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	_, err = db.Pool.Exec(context.Background(), `
		INSERT INTO curious_requests (pensiero_id, requester_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, pensieroID, user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore")
		return
	}

	// recupera ID della curious_request appena creata
	var reqID string
	db.Pool.QueryRow(context.Background(),
		`SELECT id FROM curious_requests WHERE pensiero_id=$1 AND requester_id=$2`,
		pensieroID, user.ID,
	).Scan(&reqID)

	// notifica al subject
	notify(subjectID, "curious_request", map[string]string{
		"requester_id":   user.ID,
		"requester_name": user.Username,
		"pensiero_id":    pensieroID,
		"request_id":     reqID,
	})

	// risposta HTMX: rimuove le azioni e mostra la clessidra in alto a destra
	// (posizionata in modo assoluto rispetto alla card)
	c.Data(http.StatusOK, "text/html", []byte(`
		<details class="attesa-tip">
			<summary class="attesa-btn" aria-label="In attesa di rivelazione">
				<svg class="attesa-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					<path d="M6 2h12"/><path d="M6 22h12"/>
					<path d="M7 2c0 4 3 5.5 5 7 2-1.5 5-3 5-7"/>
					<path d="M7 22c0-4 3-5.5 5-7 2 1.5 5 3 5 7"/>
				</svg>
			</summary>
			<span class="attesa-tooltip">in attesa di rivelazione...</span>
		</details>
	`))
}

func PostCuriousAccept(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	reqID := c.Param("id")

	var requesterID, pensieroID string
	err := db.Pool.QueryRow(context.Background(), `
		SELECT cr.requester_id, cr.pensiero_id
		FROM curious_requests cr
		JOIN pensieri t ON t.id = cr.pensiero_id
		WHERE cr.id = $1 AND t.subject_id = $2
	`, reqID, user.ID).Scan(&requesterID, &pensieroID)
	if err != nil {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	db.Pool.Exec(context.Background(),
		`UPDATE curious_requests SET status = 'accepted' WHERE id = $1`, reqID)

	// notifica al requester
	notify(requesterID, "curious_accepted", map[string]string{
		"pensiero_id": pensieroID,
		"accepted_by": user.Username,
	})

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
			JOIN pensieri t ON t.id = cr.pensiero_id
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
