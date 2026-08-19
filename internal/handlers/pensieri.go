package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"
	"pensieri/internal/storage"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	// Se non si segue nessuno l'elenco è vuoto e l'editor si apre già sul nome
	// libero, l'unico modo utile di scrivere un pensiero in quel momento.
	c.HTML(http.StatusOK, "pensiero-editor", gin.H{
		"Following":     following,
		"ModoLibero":    len(following) == 0,
		"MaxNomeLibero": maxLunghezzaNomeLibero,
	})
}

// AnnullaEditorPensiero svuota l'area dell'editor (risposta HTMX).
func AnnullaEditorPensiero(c *gin.Context) {
	c.Data(http.StatusOK, "text/html", []byte(""))
}

// maxLunghezzaNomeLibero limita il nome di un soggetto senza account, in linea
// con la colonna pensieri.subject_name.
const maxLunghezzaNomeLibero = 64

func PostPensiero(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	subjectID := c.PostForm("subject_id")
	subjectName := strings.TrimSpace(c.PostForm("subject_name"))
	contentPublic := strings.TrimSpace(c.PostForm("content_public"))
	contentPersonal := strings.TrimSpace(c.PostForm("content_personal"))

	// Il soggetto è un utente registrato oppure un nome libero (una persona
	// senza account), mai entrambi.
	if (subjectID == "") == (subjectName == "") {
		c.String(http.StatusBadRequest, "scegli un utente oppure scrivi un nome")
		return
	}
	if len([]rune(subjectName)) > maxLunghezzaNomeLibero {
		subjectName = string([]rune(subjectName)[:maxLunghezzaNomeLibero])
	}

	// Entrambe le versioni sono obbligatorie: si scrive sempre la versione
	// pubblica (visibile a tutti) e la versione personale (visibile solo a chi
	// la richiede con "sono curioso", o al soggetto del pensiero).
	if contentPublic == "" || contentPersonal == "" {
		c.String(http.StatusBadRequest, "servono sia la versione pubblica che quella personale")
		return
	}

	ctx := context.Background()

	pensieroID, err := upsertPensiero(ctx, user.ID, subjectID, subjectName)
	if err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio")
		return
	}

	imageURL, imageErr := uploadPensieroImageIfPresent(c, user.ID)
	if imageErr != "" {
		c.String(http.StatusBadRequest, imageErr)
		return
	}
	if imageURL != "" {
		if oldImage, err := aggiornaImmaginePensiero(ctx, pensieroID, imageURL); err != nil {
			c.String(http.StatusInternalServerError, "errore salvataggio immagine")
			return
		} else if oldImage != "" {
			deletePensieroImage(oldImage)
		}
	}

	// Versione pubblica: audience_id NULL, letta da chiunque.
	if err := salvaVersione(ctx, pensieroID, nil, false, contentPublic); err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio versione pubblica")
		return
	}

	// Versione personale: quella che gli altri utenti chiedono di vedere con
	// "sono curioso". Se il soggetto ha un account gliela indirizziamo
	// (audience_id = subject_id) così la legge senza doverla richiedere; su un
	// nome libero non c'è nessun destinatario e resta identificata dal flag.
	var audience *string
	if subjectID != "" {
		audience = &subjectID
	}
	if err := salvaVersione(ctx, pensieroID, audience, true, contentPersonal); err != nil {
		c.String(http.StatusInternalServerError, "errore salvataggio versione personale")
		return
	}

	// notifica il soggetto del pensiero (se ha un account ed è diverso dall'autore)
	if subjectID != "" && subjectID != user.ID {
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

// maxPensieroImageBytes limita l’immagine allegata a un pensiero (8 MB).
const maxPensieroImageBytes = 8 << 20

// uploadPensieroImageIfPresent accetta al massimo un file dal campo "image_file"
// e restituisce l’URL relativo /media/pensieri/<file> da salvare sul pensiero.
func uploadPensieroImageIfPresent(c *gin.Context, userID string) (string, string) {
	form, err := c.MultipartForm()
	if err != nil {
		return "", ""
	}
	files := form.File["image_file"]
	if len(files) == 0 {
		return "", ""
	}
	if len(files) > 1 {
		return "", "puoi caricare al massimo 1 immagine"
	}
	if !storage.Configured() {
		return "", "upload immagini non disponibile"
	}
	fh := files[0]
	if fh.Size > maxPensieroImageBytes {
		return "", "immagine troppo grande (max 8 MB)"
	}
	f, err := fh.Open()
	if err != nil {
		return "", "errore upload immagine"
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	contentType := http.DetectContentType(head[:n])
	ext, ok := avatarExt[contentType]
	if !ok {
		return "", "formato immagine non valido"
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", "errore upload immagine"
	}
	name := fmt.Sprintf("%s-%s.%s", userID, uuid.NewString(), ext)
	if err := storage.Upload(c.Request.Context(), "pensieri/"+name, contentType, f); err != nil {
		return "", "errore upload immagine"
	}
	return "/media/pensieri/" + name, ""
}

func aggiornaImmaginePensiero(ctx context.Context, pensieroID, imageURL string) (string, error) {
	var oldImage string
	err := db.Pool.QueryRow(ctx, `
		SELECT COALESCE(image_url, '') FROM pensieri WHERE id = $1
	`, pensieroID).Scan(&oldImage)
	if err != nil {
		return "", err
	}
	_, err = db.Pool.Exec(ctx, `
		UPDATE pensieri SET image_url = $2, updated_at = NOW() WHERE id = $1
	`, pensieroID, imageURL)
	return oldImage, err
}

func deletePensieroImage(imageURL string) {
	if storage.Configured() && strings.HasPrefix(imageURL, "/media/pensieri/") {
		_ = storage.Delete(context.Background(), "pensieri/"+strings.TrimPrefix(imageURL, "/media/pensieri/"))
	}
}

// upsertPensiero recupera (o crea) il pensiero di un autore su un soggetto e ne
// restituisce l'id. Un autore ha un solo pensiero per soggetto: riscriverlo
// aggiorna quello esistente. Il soggetto è un utente registrato (subjectID) o un
// nome libero (subjectName), che vale come chiave a meno delle maiuscole.
func upsertPensiero(ctx context.Context, authorID, subjectID, subjectName string) (string, error) {
	var pensieroID string

	if subjectID != "" {
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO pensieri (author_id, subject_id)
			VALUES ($1, $2)
			ON CONFLICT (author_id, subject_id) DO UPDATE SET updated_at = NOW()
			RETURNING id
		`, authorID, subjectID).Scan(&pensieroID)
		return pensieroID, err
	}

	// Sui nomi liberi l'unicità è garantita da un indice parziale su
	// (author_id, LOWER(subject_name)): ON CONFLICT non può inferirlo in modo
	// affidabile, quindi facciamo un upsert manuale. Riallineiamo anche il nome
	// così l'ultima grafia usata è quella che compare nelle card.
	err := db.Pool.QueryRow(ctx, `
		UPDATE pensieri SET updated_at = NOW(), subject_name = $2
		WHERE author_id = $1 AND subject_id IS NULL AND LOWER(subject_name) = LOWER($2)
		RETURNING id
	`, authorID, subjectName).Scan(&pensieroID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.Pool.QueryRow(ctx, `
			INSERT INTO pensieri (author_id, subject_name) VALUES ($1, $2) RETURNING id
		`, authorID, subjectName).Scan(&pensieroID)
	}
	return pensieroID, err
}

// salvaVersione inserisce o aggiorna una versione di un pensiero. La coppia
// (audience_id, personale) identifica la versione: audience NULL e personale
// falso è quella pubblica, personale vero è quella che si sblocca con "sono
// curioso" (indirizzata al soggetto se ha un account, senza destinatario se è un
// nome libero). Con audience_id NULL il vincolo UNIQUE non è confrontabile e
// ON CONFLICT non scatterebbe, quindi l'upsert è manuale.
func salvaVersione(ctx context.Context, pensieroID string, audienceID *string, personale bool, content string) error {
	tag, err := db.Pool.Exec(ctx, `
		UPDATE versioni_pensiero SET content = $3
		WHERE pensiero_id = $1 AND audience_id IS NOT DISTINCT FROM $2::uuid AND personale = $4
	`, pensieroID, audienceID, content, personale)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO versioni_pensiero (pensiero_id, audience_id, content, personale)
		VALUES ($1, $2, $3, $4)
	`, pensieroID, audienceID, content, personale)
	return err
}

// GetPensiero mostra un singolo pensiero "a schermo intero", su una pagina
// dedicata che contiene solo quel pensiero con il suo thread di commenti.
// È la destinazione dei link nelle notifiche (nuovo pensiero, DNA, commento…)
// e dei link condivisi (WhatsApp, Telegram…), quindi è una pagina pubblica:
// chi non ha un account ne legge la versione pubblica, esattamente come i
// pensieri che vede nel feed, senza essere dirottato sulla registrazione.
func GetPensiero(c *gin.Context) {
	user, auth := currentViewer(c)
	pensieroID := c.Param("id")

	// risolto per il viewer: l'autore vede sempre la propria versione diretta,
	// gli altri seguono la normale logica di visibilità (per un visitatore
	// anonimo si riduce sempre alla versione pubblica).
	rt, err := risolviPensiero(context.Background(), pensieroID, user.ID)
	if err != nil {
		c.HTML(http.StatusNotFound, "pensiero.html", gin.H{"User": viewerForTemplate(user, auth)})
		return
	}
	rt.CanModerate = user.IsAdmin
	// senza sessione le azioni che richiedono un account (DNA, "sono curioso",
	// commenti) diventano un invito a registrarsi.
	rt.Anon = !auth

	// il contenuto vuoto significa che il viewer non ha accesso a nessuna versione
	if rt.Content == "" {
		c.HTML(http.StatusNotFound, "pensiero.html", gin.H{"User": viewerForTemplate(user, auth)})
		return
	}

	c.HTML(http.StatusOK, "pensiero.html", gin.H{
		"User":     viewerForTemplate(user, auth),
		"Pensiero": rt,
		"Anon":     !auth,
	})
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
			`SELECT content FROM versioni_pensiero
			 WHERE pensiero_id = $1 AND audience_id IS NULL AND NOT personale`,
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

	salvaVersione(context.Background(), pensieroID, audID, false, content)

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
			`DELETE FROM versioni_pensiero
			 WHERE pensiero_id = $1 AND audience_id IS NULL AND NOT personale`, pensieroID)
	} else {
		db.Pool.Exec(context.Background(),
			`DELETE FROM versioni_pensiero WHERE pensiero_id = $1 AND audience_id = $2`, pensieroID, audienceID)
	}

	c.Status(http.StatusNoContent)
}
