package models

import "time"

type Pensiero struct {
	ID        string    `db:"id"`
	AuthorID  string    `db:"author_id"`
	SubjectID string    `db:"subject_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type VersionePensiero struct {
	ID         string    `db:"id"`
	PensieroID string    `db:"pensiero_id"`
	AudienceID *string   `db:"audience_id"` // nil = default
	Content    string    `db:"content"`
	CreatedAt  time.Time `db:"created_at"`
}

// PensieroRisolto è un pensiero già risolto per un viewer specifico
type PensieroRisolto struct {
	PensieroID  string
	AuthorID    string
	AuthorName  string
	SubjectID   string
	SubjectName string
	Content     string
	IsDirect    bool // true se il viewer sta vedendo la versione diretta (audience=subject)

	// stato richiesta "sono curioso" relativo al viewer corrente
	CanSendCurious bool // il viewer può chiedere di vedere la versione diretta
	CuriousPending bool // il viewer ha una richiesta in attesa
}
