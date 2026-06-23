package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"pensieri/internal/db"
	"pensieri/internal/models"
	"pensieri/internal/storage"

	"github.com/gin-gonic/gin"
)

// adminUserRow è una riga della tabella di amministrazione utenti: i dati
// identificativi più qualche conteggio utile a riconoscere gli account di test
// da ripulire.
type adminUserRow struct {
	ID            string
	Username      string
	Email         string
	DisplayName   string
	IsAdmin       bool
	HasUpload     bool // avatar caricato su S3 (verrà rimosso anche dal bucket)
	PensieriCount int
	CommentiCount int
	CreatedAt     time.Time
}

// GetAdmin mostra la pagina (non linkata) di amministrazione con l'elenco di
// tutti gli utenti e il pulsante di eliminazione. L'accesso è già ristretto agli
// admin dal middleware AdminOnly montato sulla rotta.
func GetAdmin(c *gin.Context) {
	me := c.MustGet("user").(models.User)

	rows, err := db.Pool.Query(context.Background(), `
		SELECT u.id, u.username, u.email,
		       COALESCE(u.display_name, ''),
		       COALESCE(u.is_admin, FALSE),
		       COALESCE(u.avatar_url, ''),
		       (SELECT COUNT(*) FROM pensieri p WHERE p.author_id = u.id),
		       (SELECT COUNT(*) FROM commenti cm WHERE cm.author_id = u.id),
		       u.created_at
		FROM users u
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore lettura utenti")
		return
	}
	defer rows.Close()

	var users []adminUserRow
	for rows.Next() {
		var u adminUserRow
		var avatarURL string
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName,
			&u.IsAdmin, &avatarURL, &u.PensieriCount, &u.CommentiCount, &u.CreatedAt); err != nil {
			continue
		}
		u.HasUpload = strings.HasPrefix(avatarURL, "/media/avatars/")
		users = append(users, u)
	}

	c.HTML(http.StatusOK, "admin.html", gin.H{
		"User":  me,
		"Users": users,
		"Me":    me.ID,
	})
}

// DeleteUser elimina definitivamente un utente e, grazie alle foreign key
// ON DELETE CASCADE, tutti i dati collegati nel DB: sessioni, follow, pensieri
// (scritti e ricevuti) con relative versioni, richieste "sono curioso",
// commenti, DNA, notifiche e token di reset password. L'unico dato fuori dal DB
// è l'eventuale avatar caricato su S3, che viene rimosso esplicitamente.
//
// L'operazione è irreversibile: pensata per ripulire account di test.
func DeleteUser(c *gin.Context) {
	me := c.MustGet("user").(models.User)
	targetID := c.Param("id")

	// Guardia: un admin non può eliminare se stesso (eviterebbe di restare senza
	// accesso e cancellerebbe la sessione corrente a metà richiesta).
	if targetID == me.ID {
		c.String(http.StatusBadRequest, "non puoi eliminare il tuo stesso account da qui")
		return
	}

	ctx := context.Background()

	// Recupera l'avatar prima di cancellare la riga, così sappiamo se c'è un
	// oggetto S3 da rimuovere. Serve anche a verificare che l'utente esista.
	var avatarURL string
	err := db.Pool.QueryRow(ctx,
		`SELECT COALESCE(avatar_url, '') FROM users WHERE id = $1`, targetID,
	).Scan(&avatarURL)
	if err != nil {
		c.String(http.StatusNotFound, "utente non trovato")
		return
	}

	// Cancellazione DB: un solo DELETE, il CASCADE fa il resto in modo atomico.
	tag, err := db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, targetID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore eliminazione utente")
		return
	}
	if tag.RowsAffected() == 0 {
		c.String(http.StatusNotFound, "utente non trovato")
		return
	}

	// Avatar su S3: best-effort. La riga DB (fonte di verità) è già stata rimossa;
	// un eventuale oggetto orfano nel bucket è innocuo, quindi un errore qui viene
	// solo loggato e non fa fallire l'operazione.
	if strings.HasPrefix(avatarURL, "/media/avatars/") && storage.Configured() {
		key := "avatars/" + strings.TrimPrefix(avatarURL, "/media/avatars/")
		if err := storage.Delete(ctx, key); err != nil {
			log.Printf("admin: avatar S3 non rimosso (%s): %v", key, err)
		}
	}

	// Risposta HTMX: rimuove la riga della tabella dal DOM.
	c.Data(http.StatusOK, "text/html", []byte(""))
}
