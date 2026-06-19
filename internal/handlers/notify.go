package handlers

import (
	"context"
	"encoding/json"
	"pensieri/internal/db"
	"pensieri/internal/email"
)

// notify registra una notifica in-app per il destinatario e, in parallelo,
// invia l'email corrispondente (se il tipo la prevede). L'invio email è
// asincrono e non blocca; eventuali errori vengono solo loggati.
func notify(recipientID, ntype string, data map[string]string) {
	payload, _ := json.Marshal(data)
	db.Pool.Exec(context.Background(),
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, $2, $3)`,
		recipientID, ntype, string(payload))

	var addr, username string
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT email, username FROM users WHERE id = $1`, recipientID,
	).Scan(&addr, &username); err != nil || addr == "" {
		return
	}

	if subject, html := email.Notification(ntype, username, data); subject != "" {
		email.SendAsync(addr, subject, html)
	}
}
