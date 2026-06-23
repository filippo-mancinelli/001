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
	CreatedAt   time.Time // istante di creazione del pensiero
	IsDirect    bool      // true se il viewer sta vedendo la versione diretta (audience=subject)

	// stato richiesta "sono curioso" relativo al viewer corrente
	CanSendCurious bool // il viewer può chiedere di vedere la versione diretta
	CuriousPending bool // il viewer ha una richiesta in attesa

	// CanToggleVersion è true quando il viewer ha accesso a entrambe le versioni
	// (pubblica e personale) e sta guardando la pubblica per scelta: serve a
	// mostrare comunque lo switch per tornare alla personale. Quando guarda la
	// personale lo switch è già implicito in IsDirect, quindi questo flag non
	// viene popolato nei render di default (resta false).
	CanToggleVersion bool

	CommentCount int // numero di commenti collegati al pensiero

	// "DNA": il like del pensiero (doppia elica). DnaCount è il totale,
	// DnaDone indica se il viewer corrente ha già lasciato il proprio DNA.
	DnaCount int
	DnaDone  bool

	// CanModerate è true se il viewer corrente è un amministratore e può quindi
	// eliminare il pensiero per moderare contenuti sensibili o vietati.
	CanModerate bool

	// Crowned è true se un amministratore ha "coronato" il pensiero: viene messo
	// in primo piano nel feed e mostrato con un bordo dorato e una corona.
	Crowned bool

	// Anon è true se il pensiero è mostrato a un visitatore non autenticato:
	// le azioni che richiedono un account (DNA, "sono curioso", commenti) vengono
	// sostituite da un invito a registrarsi.
	Anon bool
}

// Commento è un commento di un utente su un pensiero, già risolto per la vista
// (include il nome dell'autore e se il viewer corrente può eliminarlo).
type Commento struct {
	ID           string    `db:"id"`
	PensieroID   string    `db:"pensiero_id"`
	AuthorID     string    `db:"author_id"`
	AuthorName   string    // username dell'autore del commento
	AuthorAvatar string    // avatar_url dell'autore (vuoto = fallback iniziale)
	Content      string    `db:"content"`
	CreatedAt    time.Time `db:"created_at"`

	// CanDelete è true se il viewer può eliminare il commento: ne è l'autore
	// oppure è l'autore del pensiero (modera i commenti sotto il proprio pensiero).
	CanDelete bool
}
