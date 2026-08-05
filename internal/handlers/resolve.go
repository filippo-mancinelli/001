package handlers

import (
	"context"
	"pensieri/internal/db"
	"pensieri/internal/models"

	"github.com/gin-gonic/gin"
)

// anonViewerID è l'UUID "nullo" usato come viewer per i visitatori non
// autenticati: è un UUID valido (le query non vanno in errore) ma non
// corrisponde a nessun utente reale, quindi le regole di visibilità risolvono
// sempre la versione pubblica (audience_id IS NULL) e non mostrano mai versioni
// personali o azioni riservate.
const anonViewerID = "00000000-0000-0000-0000-000000000000"

// currentViewer restituisce l'utente loggato (se presente) e un flag che indica
// se la richiesta è autenticata. Per i visitatori anonimi ritorna un User con ID
// uguale ad anonViewerID, così i frammenti SQL di risoluzione restano validi.
func currentViewer(c *gin.Context) (models.User, bool) {
	if v, ok := c.Get("user"); ok {
		return v.(models.User), true
	}
	return models.User{ID: anonViewerID}, false
}

// Questo file contiene i frammenti SQL condivisi che risolvono un pensiero per
// uno specifico viewer. Il viewer è passato come placeholder posizionale
// (es. "$1" nel feed, "$2" nella pagina profilo) così le query possono
// riutilizzare la stessa logica senza duplicarla.
//
// Regole di risoluzione del contenuto, in ordine di priorità:
//  1. esiste una versione scritta apposta per il viewer (audience_id = viewer)
//  2. il viewer ha una richiesta "sono curioso" accettata -> vede la versione
//     personale
//  3. altrimenti la versione pubblica
//
// Il soggetto di un pensiero può essere un utente registrato (subject_id) o un
// nome libero, cioè una persona senza account (subject_name). Nel secondo caso
// la versione personale non ha un destinatario a cui indirizzarla, quindi è
// riconoscibile solo dal flag "personale" (vedi migrazione 009).

// versionePersonale è la sottoquery del contenuto della versione personale,
// quella che si sblocca con "sono curioso".
const versionePersonale = `(SELECT content FROM versioni_pensiero
	WHERE pensiero_id = t.id AND personale)`

// versionePubblica è la sottoquery del contenuto della versione pubblica,
// quella che leggono tutti.
const versionePubblica = `(SELECT content FROM versioni_pensiero
	WHERE pensiero_id = t.id AND audience_id IS NULL AND NOT personale)`

// esistePersonale indica se il pensiero ha una versione personale.
const esistePersonale = `EXISTS (SELECT 1 FROM versioni_pensiero
	WHERE pensiero_id = t.id AND personale)`

// colonneIdentita elenca le colonne che identificano autore e soggetto di un
// pensiero, nell'ordine atteso da scanPensiero. Se il soggetto è un nome libero
// subject_id è la stringa vuota e il nome arriva da t.subject_name.
const colonneIdentita = `t.id, t.author_id, u_a.username,
	COALESCE(t.subject_id::text, '') AS subject_id,
	COALESCE(u_s.username, t.subject_name, '') AS subject_name`

// joinUtenti collega autore e soggetto. Il soggetto è in LEFT JOIN perché i
// pensieri scritti su un nome libero non hanno un utente collegato.
const joinUtenti = `JOIN users u_a ON u_a.id = t.author_id
	LEFT JOIN users u_s ON u_s.id = t.subject_id`

// curiositaAccettata indica se il viewer ha ottenuto l'accesso alla versione
// personale tramite una richiesta "sono curioso" accettata.
func curiositaAccettata(viewer string) string {
	return `EXISTS (SELECT 1 FROM curious_requests cr
		WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'accepted')`
}

// resolviContenuto restituisce l'espressione COALESCE per il contenuto.
func resolviContenuto(viewer string) string {
	return `COALESCE(
		(SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `),
		CASE WHEN ` + curiositaAccettata(viewer) + ` THEN ` + versionePersonale + ` END,
		` + versionePubblica + `
	)`
}

// isDiretta indica se il viewer sta vedendo la versione personale, perché ne è
// il soggetto o perché la sua richiesta di curiosità è stata accettata.
func isDiretta(viewer string) string {
	return `(
		(t.subject_id IS NOT NULL AND t.subject_id = ` + viewer + ` AND ` + esistePersonale + `)
		OR (
			` + curiositaAccettata(viewer) + `
			AND NOT EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `)
			AND ` + esistePersonale + `
		)
	)`
}

// puoCurioso indica se il viewer può inviare una richiesta "sono
// curioso": non è autore né soggetto, esiste una versione personale che non sta
// già vedendo, e non ha ancora inviato alcuna richiesta per questo pensiero.
func puoCurioso(viewer string) string {
	return `(
		t.author_id <> ` + viewer + `
		AND (t.subject_id IS NULL OR t.subject_id <> ` + viewer + `)
		AND ` + esistePersonale + `
		AND NOT EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `)
		AND NOT EXISTS (SELECT 1 FROM curious_requests cr
			WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + `)
	)`
}

// puoRivelare indica se il viewer può accettare o rifiutare una richiesta "sono
// curioso" sul pensiero: è il soggetto se ha un account, altrimenti — sui nomi
// liberi, dove nessuno può parlare per il soggetto — l'autore del pensiero.
func puoRivelare(viewer string) string {
	return `(
		t.subject_id = ` + viewer + `
		OR (t.subject_id IS NULL AND t.author_id = ` + viewer + `)
	)`
}

// curiosoInAttesa indica se il viewer ha una richiesta ancora in attesa.
func curiosoInAttesa(viewer string) string {
	return `EXISTS (SELECT 1 FROM curious_requests cr
		WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'pending')`
}

// contaCommenti restituisce il numero di commenti collegati al pensiero.
func contaCommenti() string {
	return `(SELECT COUNT(*) FROM commenti WHERE pensiero_id = t.id)`
}

// contaDna restituisce il numero di "DNA" (like) lasciati sul pensiero.
func contaDna() string {
	return `(SELECT COUNT(*) FROM dna_likes WHERE pensiero_id = t.id)`
}

// dnaFatto indica se il viewer ha già lasciato il proprio DNA sul pensiero.
func dnaFatto(viewer string) string {
	return `EXISTS (SELECT 1 FROM dna_likes WHERE pensiero_id = t.id AND user_id = ` + viewer + `)`
}

// colonneRisolte assembla le colonne risolte (content, is_direct,
// can_send_curious, curious_pending, comment_count, dna_count, dna_done,
// created_at) per un viewer, nell'ordine atteso dallo scan dei PensieroRisolto.
func colonneRisolte(viewer string) string {
	return resolviContenuto(viewer) + ` AS content,
		` + isDiretta(viewer) + ` AS is_direct,
		` + puoCurioso(viewer) + ` AS can_send_curious,
		` + curiosoInAttesa(viewer) + ` AS curious_pending,
		` + contaCommenti() + ` AS comment_count,
		` + contaDna() + ` AS dna_count,
		` + dnaFatto(viewer) + ` AS dna_done,
		t.created_at AS created_at,
		t.crowned AS crowned`
}

// colonneAutore assembla le colonne risolte dal punto di vista di chi potrebbe
// essere l'autore del pensiero: l'autore vede sempre tutto ciò che ha scritto
// (versione personale inclusa), gli altri seguono le regole standard.
func colonneAutore(viewer string) string {
	return `CASE WHEN t.author_id = ` + viewer + ` THEN
			COALESCE(` + versionePersonale + `, ` + versionePubblica + `)
		ELSE ` + resolviContenuto(viewer) + ` END AS content,
		(t.author_id = ` + viewer + ` OR ` + isDiretta(viewer) + `) AS is_direct,
		(t.author_id <> ` + viewer + ` AND ` + puoCurioso(viewer) + `) AS can_send_curious,
		` + curiosoInAttesa(viewer) + ` AS curious_pending,
		` + contaCommenti() + ` AS comment_count,
		` + contaDna() + ` AS dna_count,
		` + dnaFatto(viewer) + ` AS dna_done,
		t.created_at AS created_at,
		t.crowned AS crowned`
}

// rowScanner è soddisfatto sia da pgx.Row che da pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanPensiero legge una riga nell'ordine di colonneIdentita seguite da
// colonneRisolte (o colonneAutore, che espone le stesse colonne).
func scanPensiero(row rowScanner) (models.PensieroRisolto, error) {
	var rt models.PensieroRisolto
	err := row.Scan(&rt.PensieroID, &rt.AuthorID, &rt.AuthorName, &rt.SubjectID, &rt.SubjectName,
		&rt.Content, &rt.IsDirect, &rt.CanSendCurious, &rt.CuriousPending, &rt.CommentCount,
		&rt.DnaCount, &rt.DnaDone, &rt.CreatedAt, &rt.Crowned)
	return rt, err
}

// risolviPensiero risolve un singolo pensiero per uno specifico viewer, con la
// stessa logica di visibilità della pagina dedicata (l'autore vede sempre la
// propria versione diretta, gli altri seguono le regole standard). È usata sia
// dalla pagina del singolo pensiero sia per ri-renderizzare la card dopo aver
// coronato/decoronato un pensiero. Restituisce un errore se il pensiero non
// esiste o se il viewer non ha accesso ad alcuna versione (content vuoto).
func risolviPensiero(ctx context.Context, pensieroID, viewerID string) (models.PensieroRisolto, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT `+colonneIdentita+`,
			`+colonneAutore("$2")+`
		FROM pensieri t
		`+joinUtenti+`
		WHERE t.id = $1
	`, pensieroID, viewerID)

	return scanPensiero(row)
}
