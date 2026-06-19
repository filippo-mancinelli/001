package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

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

	tag, _ := db.Pool.Exec(context.Background(),
		`INSERT INTO follows (follower_id, following_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		user.ID, targetID,
	)

	// notifica al seguito solo se è un follow nuovo (e non sé stesso)
	if tag.RowsAffected() > 0 && targetID != user.ID {
		notify(targetID, "new_follower", map[string]string{
			"follower_id":   user.ID,
			"follower_name": user.Username,
		})
	}

	// risposta HTMX: bottone aggiornato
	c.Data(http.StatusOK, "text/html", []byte(`
		<button id="follow-btn" class="btn btn-red btn-sm"
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
