package handlers

import (
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
//     diretta (audience_id = subject)
//  3. altrimenti la versione di default (audience_id IS NULL)

// resolviContenuto restituisce l'espressione COALESCE per il contenuto.
func resolviContenuto(viewer string) string {
	return `COALESCE(
		(SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `),
		CASE WHEN EXISTS (
			SELECT 1 FROM curious_requests cr
			WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'accepted'
		) THEN (SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = t.subject_id) END,
		(SELECT content FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id IS NULL)
	)`
}

// isDiretta indica se il viewer sta vedendo la versione diretta
// (quella indirizzata al subject), perché ne è il subject o perché la sua
// richiesta di curiosità è stata accettata.
func isDiretta(viewer string) string {
	return `(
		(t.subject_id = ` + viewer + ` AND EXISTS (
			SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = t.subject_id))
		OR (
			EXISTS (SELECT 1 FROM curious_requests cr
				WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'accepted')
			AND NOT EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `)
			AND EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = t.subject_id)
		)
	)`
}

// puoCurioso indica se il viewer può inviare una richiesta "sono
// curioso": non è autore né subject, esiste una versione diretta che non sta
// già vedendo, e non ha ancora inviato alcuna richiesta per questo pensiero.
func puoCurioso(viewer string) string {
	return `(
		t.author_id <> ` + viewer + ` AND t.subject_id <> ` + viewer + `
		AND EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = t.subject_id)
		AND NOT EXISTS (SELECT 1 FROM versioni_pensiero WHERE pensiero_id = t.id AND audience_id = ` + viewer + `)
		AND NOT EXISTS (SELECT 1 FROM curious_requests cr
			WHERE cr.pensiero_id = t.id AND cr.requester_id = ` + viewer + `)
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
		t.created_at AS created_at`
}
