package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

type notifView struct {
	models.Notification
	Message     string
	MessageHTML template.HTML
	Actions     []notifAction
}

type notifAction struct {
	Label string
	URL   string
	Class string
}

func GetNotificationsCount(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	var count int
	db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notifications
		WHERE user_id = $1 AND read = FALSE
	`, user.ID).Scan(&count)

	if count == 0 {
		c.String(http.StatusOK, "")
		return
	}
	label := fmt.Sprintf("%d", count)
	if count > 99 {
		label = "99+"
	}
	c.String(http.StatusOK, `<span class="notif-badge">`+label+`</span>`)
}

func GetNotifications(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT id, user_id, type, payload, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, user.ID)
	if err != nil {
		c.HTML(http.StatusOK, "notifications.html", gin.H{"User": user})
		return
	}
	defer rows.Close()

	var notifs []notifView
	for rows.Next() {
		var n models.Notification
		rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.Read, &n.CreatedAt)

		nv := notifView{Notification: n}
		var p map[string]string
		json.Unmarshal(n.Payload, &p)

		mention := func(u string) string {
			return `<span class="notif-mention">@` + u + `</span>`
		}
		const dnaIcon = `<svg style="display:inline;vertical-align:middle;margin:0 2px" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 2c3 3 5 7 5 12"/><path d="M22 2c-3 3-5 7-5 12"/><path d="M2 22c3-3 5-7 5-12"/><path d="M22 22c-3-3-5-7-5-12"/><path d="M6 12h12"/><path d="M7 8h10"/><path d="M7 16h10"/></svg>`
		var msgHTML string
		switch n.Type {
		case "curious_request":
			msgHTML = mention(p["requester_name"]) + " vuole sapere cosa pensi davvero di te"
			nv.Actions = []notifAction{
				{Label: "[ accetta ]", URL: "/curious/" + p["request_id"] + "/accept", Class: "btn-cyan"},
				{Label: "[ rifiuta ]", URL: "/curious/" + p["request_id"] + "/reject", Class: "btn-red"},
			}
		case "curious_accepted":
			msgHTML = mention(p["accepted_by"]) + " ha accettato — puoi ora vedere il pensiero diretto"
		case "dna":
			msgHTML = mention(p["liker_name"]) + " ha trovato un'affinità genetica " + dnaIcon + " con un tuo pensiero"
		case "new_comment":
			msgHTML = mention(p["commenter_name"]) + " ha commentato un tuo pensiero"
		case "new_thought":
			msgHTML = mention(p["author_name"]) + " ha espresso un pensiero su di te"
		case "new_follower":
			msgHTML = mention(p["follower_name"]) + " ha iniziato a seguirti"
		default:
			msgHTML = n.Type
		}
		nv.MessageHTML = template.HTML(msgHTML)
		nv.Message = msgHTML

		notifs = append(notifs, nv)
	}

	// segna come lette
	db.Pool.Exec(context.Background(),
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`, user.ID)

	c.HTML(http.StatusOK, "notifications.html", gin.H{"User": user, "Notifications": notifs})
}
