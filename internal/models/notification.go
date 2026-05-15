package models

import (
	"encoding/json"
	"time"
)

type Notification struct {
	ID        string          `db:"id"`
	UserID    string          `db:"user_id"`
	Type      string          `db:"type"`
	Payload   json.RawMessage `db:"payload"`
	Read      bool            `db:"read"`
	CreatedAt time.Time       `db:"created_at"`
}

type CuriousRequest struct {
	ID          string    `db:"id"`
	ThoughtID   string    `db:"thought_id"`
	RequesterID string    `db:"requester_id"`
	Status      string    `db:"status"` // pending | accepted | rejected
	CreatedAt   time.Time `db:"created_at"`
}
