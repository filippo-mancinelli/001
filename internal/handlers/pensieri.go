package handlers

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetEditorPensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	rows, _ := db.Pool.Query(context.Background(),
		`SELECT u.id, u.username FROM follows f JOIN users u ON u.id = f.following_id WHERE f.follower_id = $1`,
		user.ID,
	)
	defer rows.Close()

	var following []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Username)
		following = append(following, u)
	}

	c.HTML(http.StatusOK, "pensiero-editor", gin.H{"Following": following})
}

// AnnullaEditorPensiero svuota l'area dell'editor (risposta HTMX).
func AnnullaEditorPensiero(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(""))
}

func PostPensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	subjectID := c.PostForm("subject_id")
	contentPublic := strings.TrimSpace(c.PostForm("content_public"))
	contentPersonal := strings.TrimSpace(c.PostForm("content_personal"))

	// Entrambe le versioni sono obbligatorie: si scrive sempre la versione
	// pubblica (visibile a tutti) e la versione personale (visibile solo a chi
	// la richiede con "sono curioso", o al soggetto del pensiero).
	if subjectID == "" || contentPublic == "" || contentPersonal == "" {
		c.String(http.StatusBadRequest, "servono sia la versione pubblica che quella personale")
		return
	}

	ctx := context.Background()

	// upsert pensiero meta
	var pensieroID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO pensieri (author_id, subject_id)
		VALUES ($1, $2)
		ON CONFLICT (author_id, subject_id) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`, user.ID, subjectID).Scan(&pensieroID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio")
		return
	}

	// Versione pubblica (audience_id IS NULL). NULL non è confrontabile dal
	// constraint UNIQUE, quindi ON CONFLICT non scatterebbe: facciamo un upsert
	// manuale (UPDATE, e in mancanza di righe l'INSERT) per non duplicare la riga.
	tag, err := db.Pool.Exec(ctx,
		`UPDATE versioni_pensiero SET content = $2 WHERE pensiero_id = $1 AND audience_id IS NULL`,
		pensieroID, contentPublic)
	if err == nil && tag.RowsAffected() == 0 {
		_, err = db.Pool.Exec(ctx,
			`INSERT INTO versioni_pensiero (pensiero_id, audience_id, content) VALUES ($1, NULL, $2)`,
			pensieroID, contentPublic)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio versione pubblica")
		return
	}

	// Versione personale: la memorizziamo con audience_id = subject_id. È la
	// versione che gli altri utenti chiedono di vedere con "sono curioso".
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO versioni_pensiero (pensiero_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (pensiero_id, audience_id) DO UPDATE SET content = $3
	`, pensieroID, subjectID, contentPersonal)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio versione personale")
		return
	}

	// notifica il soggetto del pensiero (se diverso dall'autore)
	if subjectID != user.ID {
		notify(subjectID, "new_thought", map[string]string{
			"author_name": user.Username,
			"pensiero_id": pensieroID,
		})
	}

	// risposta HTMX: svuota editor con messaggio conferma
	c.Data(http.StatusOK, "text/html", []byte(`
		<div style="color:var(--green); font-size:0.8rem; padding:0.5rem 0">
			[ pensiero salvato ] — <a href="">ricarica la pagina</a>
		</div>
	`))
}

// GetPensiero mostra un singolo pensiero "a schermo intero", su una pagina
// dedicata che contiene solo quel pensiero con il suo thread di commenti.
// È la destinazione dei link nelle notifiche (nuovo pensiero, DNA, commento…),
// utile per raggiungere direttamente un pensiero anche quando il thread è lungo.
func GetPensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")

	// risolto per il viewer: l'autore vede sempre la propria versione diretta,
	// gli altri seguono la normale logica di visibilità.
	rt, err := risolviPensiero(context.Background(), pensieroID, user.ID)
	if err != nil {
		c.HTML(http.StatusNotFound, "pensiero.html", gin.H{"User": user})
		return
	}
	rt.CanModerate = user.IsAdmin

	// il contenuto vuoto significa che il viewer non ha accesso a nessuna versione
	if rt.Content == "" {
		c.HTML(http.StatusNotFound, "pensiero.html", gin.H{"User": user})
		return
	}

	c.HTML(http.StatusOK, "pensiero.html", gin.H{"User": user, "Pensiero": rt})
}

// GetPensieroVersione ri-renderizza la card di un pensiero mostrando una
// versione specifica (pubblica o personale). È riservata ai viewer che hanno
// accesso a entrambe le versioni — l'autore, il soggetto del pensiero o chi ha
// avuto accettata la richiesta "sono curioso" — e permette di alternare le due
// senza perdere l'accesso a quella pubblica dopo aver sbloccato la personale.
func GetPensieroVersione(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	pensieroID := c.Param("id")
	vista := c.Query("vista")

	rt, err := risolviPensiero(context.Background(), pensieroID, user.ID)
	if err != nil || rt.Content == "" {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	// Solo chi vede la versione diretta (autore, soggetto o curioso accettato)
	// può alternare le versioni: per gli altri la pubblica è l'unica visibile.
	if !rt.IsDirect {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	rt.CanModerate = user.IsAdmin
	rt.CanToggleVersion = true

	if vista == "pubblica" {
		var pub string
		if err := db.Pool.QueryRow(context.Background(),
			`SELECT content FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id IS NULL`,
			pensieroID).Scan(&pub); err != nil {
			c.String(http.StatusNotFound, "versione pubblica non trovata")
			return
		}
		rt.Content = pub
		rt.IsDirect = false
	}
	// vista "personale" (default): rt è già risolto sulla versione diretta.

	c.HTML(http.StatusOK, "pensiero-card", rt)
}

func PostVersionePensiero(c *gin.Context) {
	pensieroID := c.Param("id")
	audienceID := c.PostForm("audience_id")
	content := c.PostForm("content")

	var audID *string
	if audienceID != "" {
		audID = &audienceID
	}

	db.Pool.Exec(context.Background(), `
		INSERT INTO versioni_pensiero (pensiero_id, audience_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (pensiero_id, audience_id) DO UPDATE SET content = $3
	`, pensieroID, audID, content)

	c.Status(http.StatusNoContent)
}

// DeletePensiero elimina un intero pensiero (e a cascata versioni, commenti,
// richieste di curiosità e DNA). Riservato agli amministratori per moderare
// contenuti sensibili o vietati. Risponde con HTML vuoto così l'elemento card
// può essere rimosso dal feed via HTMX (hx-swap="outerHTML").
func DeletePensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if !user.IsAdmin {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	pensieroID := c.Param("id")
	tag, err := db.Pool.Exec(context.Background(),
		`DELETE FROM pensieri WHERE id = $1`, pensieroID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore eliminazione")
		return
	}
	if tag.RowsAffected() == 0 {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	c.Data(http.StatusOK, "text/html", []byte(""))
}

// PostCorona "corona" o decorona un pensiero (toggle). Riservato agli
// amministratori: un pensiero coronato finisce in primo piano nel feed e viene
// mostrato con un bordo dorato e una corona. Risponde con la card aggiornata
// così HTMX può sostituirla in place (hx-swap="outerHTML").
func PostCorona(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	if !user.IsAdmin {
		c.String(http.StatusForbidden, "non autorizzato")
		return
	}

	pensieroID := c.Param("id")
	ctx := context.Background()

	var crowned bool
	if err := db.Pool.QueryRow(ctx,
		`UPDATE pensieri SET crowned = NOT crowned WHERE id = $1 RETURNING crowned`,
		pensieroID).Scan(&crowned); err != nil {
		c.String(http.StatusNotFound, "pensiero non trovato")
		return
	}

	// ri-renderizza la card aggiornata (bordo dorato + corona o rimozione).
	rt, err := risolviPensiero(ctx, pensieroID, user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore aggiornamento")
		return
	}
	rt.CanModerate = user.IsAdmin

	c.HTML(http.StatusOK, "pensiero-card", rt)
}

func EliminaVersionePensiero(c *gin.Context) {
	pensieroID := c.Param("id")
	audienceID := c.Param("audienceID")

	if audienceID == "default" {
		db.Pool.Exec(context.Background(),
			`DELETE FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id IS NULL`, pensieroID)
	} else {
		db.Pool.Exec(context.Background(),
			`DELETE FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id = $2`, pensieroID, audienceID)
	}

	c.Status(http.StatusNoContent)
}
