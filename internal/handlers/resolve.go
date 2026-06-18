package handlers

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

// thoughtResolveContent restituisce l'espressione COALESCE per il contenuto.
func thoughtResolveContent(viewer string) string {
	return `COALESCE(
		(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id = ` + viewer + `),
		CASE WHEN EXISTS (
			SELECT 1 FROM curious_requests cr
			WHERE cr.thought_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'accepted'
		) THEN (SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id = t.subject_id) END,
		(SELECT content FROM thought_versions WHERE thought_id = t.id AND audience_id IS NULL)
	)`
}

// thoughtIsDirect indica se il viewer sta vedendo la versione diretta
// (quella indirizzata al subject), perché ne è il subject o perché la sua
// richiesta di curiosità è stata accettata.
func thoughtIsDirect(viewer string) string {
	return `(
		(t.subject_id = ` + viewer + ` AND EXISTS (
			SELECT 1 FROM thought_versions WHERE thought_id = t.id AND audience_id = t.subject_id))
		OR (
			EXISTS (SELECT 1 FROM curious_requests cr
				WHERE cr.thought_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'accepted')
			AND NOT EXISTS (SELECT 1 FROM thought_versions WHERE thought_id = t.id AND audience_id = ` + viewer + `)
			AND EXISTS (SELECT 1 FROM thought_versions WHERE thought_id = t.id AND audience_id = t.subject_id)
		)
	)`
}

// thoughtCanSendCurious indica se il viewer può inviare una richiesta "sono
// curioso": non è autore né subject, esiste una versione diretta che non sta
// già vedendo, e non ha ancora inviato alcuna richiesta per questo pensiero.
func thoughtCanSendCurious(viewer string) string {
	return `(
		t.author_id <> ` + viewer + ` AND t.subject_id <> ` + viewer + `
		AND EXISTS (SELECT 1 FROM thought_versions WHERE thought_id = t.id AND audience_id = t.subject_id)
		AND NOT EXISTS (SELECT 1 FROM thought_versions WHERE thought_id = t.id AND audience_id = ` + viewer + `)
		AND NOT EXISTS (SELECT 1 FROM curious_requests cr
			WHERE cr.thought_id = t.id AND cr.requester_id = ` + viewer + `)
	)`
}

// thoughtCuriousPending indica se il viewer ha una richiesta ancora in attesa.
func thoughtCuriousPending(viewer string) string {
	return `EXISTS (SELECT 1 FROM curious_requests cr
		WHERE cr.thought_id = t.id AND cr.requester_id = ` + viewer + ` AND cr.status = 'pending')`
}

// thoughtResolveCols assembla le colonne risolte (content, is_direct,
// can_send_curious, curious_pending) per un viewer, nell'ordine atteso dallo
// scan dei ResolvedThought.
func thoughtResolveCols(viewer string) string {
	return thoughtResolveContent(viewer) + ` AS content,
		` + thoughtIsDirect(viewer) + ` AS is_direct,
		` + thoughtCanSendCurious(viewer) + ` AS can_send_curious,
		` + thoughtCuriousPending(viewer) + ` AS curious_pending`
}
