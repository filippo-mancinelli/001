package models

import "time"

type Thought struct {
	ID        string    `db:"id"`
	AuthorID  string    `db:"author_id"`
	SubjectID string    `db:"subject_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type ThoughtVersion struct {
	ID         string    `db:"id"`
	ThoughtID  string    `db:"thought_id"`
	AudienceID *string   `db:"audience_id"` // nil = default
	Content    string    `db:"content"`
	CreatedAt  time.Time `db:"created_at"`
}

// ResolvedThought è un pensiero già risolto per un viewer specifico
type ResolvedThought struct {
	ThoughtID   string
	AuthorID    string
	AuthorName  string
	SubjectID   string
	SubjectName string
	Content     string
	IsDirect    bool // true se il viewer sta vedendo la versione diretta (audience=subject)
}
