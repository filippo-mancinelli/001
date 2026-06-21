package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

type notifView struct {
	models.Notification
	Message string
	Actions []notifAction
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

		switch n.Type {
		case "curious_request":
			nv.Message = fmt.Sprintf("@%s vuole sapere cosa pensi davvero di te", p["requester_name"])
			nv.Actions = []notifAction{
				{Label: "[ accetta ]", URL: "/curious/" + p["request_id"] + "/accept", Class: "btn-cyan"},
				{Label: "[ rifiuta ]", URL: "/curious/" + p["request_id"] + "/reject", Class: "btn-red"},
			}
		case "curious_accepted":
			nv.Message = fmt.Sprintf("@%s ha accettato — puoi ora vedere il pensiero diretto", p["accepted_by"])
		case "new_comment":
			nv.Message = fmt.Sprintf("@%s ha commentato un tuo pensiero", p["commenter_name"])
		case "new_follower":
			nv.Message = fmt.Sprintf("@%s ha iniziato a seguirti", p["follower_name"])
		default:
			nv.Message = n.Type
		}

		notifs = append(notifs, nv)
	}

	// segna come lette
	db.Pool.Exec(context.Background(),
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`, user.ID)

	c.HTML(http.StatusOK, "notifications.html", gin.H{"User": user, "Notifications": notifs})
}
