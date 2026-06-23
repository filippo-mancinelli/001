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
	IsAdmin      bool      `db:"is_admin"`
	CreatedAt    time.Time `db:"created_at"`
}

// PresenceWindow è la finestra entro cui un utente è considerato "online": se
// l'ultimo battito (last_seen) è più recente di così è online, altrimenti offline.
// Deve essere maggiore dell'intervallo di heartbeat del client (~30s) per
// tollerare un battito perso senza far "lampeggiare" lo stato.
const PresenceWindow = "75 seconds"

// PresenceExpr è l'espressione SQL che deriva lo stato di presenza reale di un
// utente dal suo ultimo battito (last_seen), restituendo 'online' oppure
// 'offline'. prefix è l'alias di tabella (es. "u"). Sostituisce il vecchio campo
// presence impostato a mano: lo stato è ora automatico e riflette l'attività reale.
func PresenceExpr(prefix string) string {
	p := ""
	if prefix != "" {
		p = prefix + "."
	}
	return "CASE WHEN " + p + "last_seen > NOW() - INTERVAL '" + PresenceWindow +
		"' THEN 'online' ELSE 'offline' END"
}

// UserColumns elenca le colonne profilo nell'ordine atteso da ScanUser, con
// COALESCE per i campi testuali nullabili. Il prefisso è la tabella/alias
// (es. "u" -> "u.id, u.username, ..."). La presenza è derivata da last_seen
// (vedi PresenceExpr), non più letta da un campo impostato a mano.
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
		PresenceExpr(prefix) + ", " +
		"COALESCE(" + p + "location, ''), " +
		"COALESCE(" + p + "website, ''), " +
		"COALESCE(" + p + "is_admin, FALSE), " +
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
		&u.AvatarURL, &u.Status, &u.Presence, &u.Location, &u.Website, &u.IsAdmin, &u.CreatedAt,
	)
}

// Display restituisce il nome visualizzato se impostato, altrimenti lo username.
func (u User) Display() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}
