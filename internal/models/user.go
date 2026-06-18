package models

import "time"

type User struct {
	ID           string    `db:"id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Bio          string    `db:"bio"`
	DisplayName  string    `db:"display_name"`
	AvatarURL    string    `db:"avatar_url"`
	Status       string    `db:"status"`
	Presence     string    `db:"presence"`
	Location     string    `db:"location"`
	Website      string    `db:"website"`
	CreatedAt    time.Time `db:"created_at"`
}

// UserColumns elenca le colonne profilo nell'ordine atteso da ScanUser, con
// COALESCE per i campi testuali nullabili. Il prefisso è la tabella/alias
// (es. "u" -> "u.id, u.username, ...").
func UserColumns(prefix string) string {
	p := ""
	if prefix != "" {
		p = prefix + "."
	}
	return p + "id, " + p + "username, " + p + "email, " +
		"COALESCE(" + p + "bio, ''), " +
		"COALESCE(" + p + "display_name, ''), " +
		"COALESCE(" + p + "avatar_url, ''), " +
		"COALESCE(" + p + "status, ''), " +
		"COALESCE(" + p + "presence, 'online'), " +
		"COALESCE(" + p + "location, ''), " +
		"COALESCE(" + p + "website, ''), " +
		p + "created_at"
}

// scanner è soddisfatto sia da pgx.Row che da pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// ScanUser legge le colonne nell'ordine di UserColumns.
func ScanUser(row scanner, u *User) error {
	return row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Bio, &u.DisplayName,
		&u.AvatarURL, &u.Status, &u.Presence, &u.Location, &u.Website, &u.CreatedAt,
	)
}

// Display restituisce il nome visualizzato se impostato, altrimenti lo username.
func (u User) Display() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}
