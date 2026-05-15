package handlers

import (
	"context"
	"net/http"
	"thoughts/internal/db"
	"thoughts/internal/models"

	"github.com/gin-gonic/gin"
)

func PostFollow(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	username := c.Param("username")

	var targetID string
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE username = $1`, username,
	).Scan(&targetID)
	if err != nil {
		c.String(http.StatusNotFound, "utente non trovato")
		return
	}

	db.Pool.Exec(context.Background(),
		`INSERT INTO follows (follower_id, following_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		user.ID, targetID,
	)

	// risposta HTMX: bottone aggiornato
	c.Data(http.StatusOK, "text/html", []byte(`
		<button class="btn btn-red btn-sm"
			hx-delete="/follow/`+username+`"
			hx-target="#follow-btn"
			hx-swap="outerHTML">
			[ smetti di seguire ]
		</button>
	`))
}

func DeleteFollow(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	username := c.Param("username")

	var targetID string
	db.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE username = $1`, username,
	).Scan(&targetID)

	db.Pool.Exec(context.Background(),
		`DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`,
		user.ID, targetID,
	)

	c.Data(http.StatusOK, "text/html", []byte(`
		<button id="follow-btn" class="btn btn-cyan btn-sm"
			hx-post="/follow/`+username+`"
			hx-target="#follow-btn"
			hx-swap="outerHTML">
			[ + segui ]
		</button>
	`))
}
