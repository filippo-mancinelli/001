package email

import (
	"fmt"
	"html/template"
	"strings"
)

// layout avvolge il corpo del messaggio in un guscio HTML coerente con
// l'estetica "aqua" dell'app, con un eventuale bottone call-to-action.
func layout(heading, intro, ctaLabel, ctaURL, footer string) string {
	var cta string
	if ctaLabel != "" && ctaURL != "" {
		cta = fmt.Sprintf(`
		<tr><td style="padding:8px 0 4px">
			<a href="%s" style="display:inline-block;background:#3b85d6;color:#ffffff;
				text-decoration:none;font-weight:700;font-size:14px;padding:11px 22px;
				border-radius:6px;border:1px solid #235f9f">%s</a>
		</td></tr>`, template.HTMLEscapeString(ctaURL), template.HTMLEscapeString(ctaLabel))
	}
	if footer == "" {
		footer = "Hai ricevuto questa email perché sei iscritto a pensieri."
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="it"><body style="margin:0;background:#c3d6ec;font-family:'Segoe UI',Tahoma,Arial,sans-serif;color:#2b3038">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 12px">
<tr><td align="center">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:480px;background:#ffffff;border:1px solid #9fb6cf;border-radius:9px;overflow:hidden">
    <tr><td style="background:linear-gradient(180deg,#5f99da,#2f6cae);padding:14px 24px;color:#fff;font-weight:700;font-size:15px;letter-spacing:0.04em">[ pensieri ]</td></tr>
    <tr><td style="padding:24px">
      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
        <tr><td style="font-size:18px;font-weight:700;padding-bottom:10px">%s</td></tr>
        <tr><td style="font-size:14px;line-height:1.6;color:#3a4654">%s</td></tr>%s
      </table>
    </td></tr>
    <tr><td style="border-top:1px solid #dde6f0;padding:14px 24px;font-size:11px;color:#8c99a8">%s</td></tr>
  </table>
</td></tr>
</table>
</body></html>`, template.HTMLEscapeString(heading), intro, cta, template.HTMLEscapeString(footer))
}

// Welcome compone l'email di benvenuto. Ritorna oggetto e corpo HTML.
func Welcome(username string) (subject, html string) {
	subject = "Benvenuto su pensieri"
	body := fmt.Sprintf("Ciao <strong>@%s</strong>,<br><br>il tuo nodo nella rete è attivo. "+
		"Inizia a seguire persone e a scrivere i tuoi pensieri.", template.HTMLEscapeString(username))
	html = layout("Benvenuto nella rete", body, "Vai al feed", AppURL()+"/", "")
	return
}

// PasswordReset compone l'email di reset password con link tokenizzato.
func PasswordReset(username, token string) (subject, html string) {
	subject = "Reimposta la tua password"
	link := AppURL() + "/reset?token=" + token
	body := fmt.Sprintf("Ciao <strong>@%s</strong>,<br><br>abbiamo ricevuto una richiesta di reimpostazione "+
		"della password. Il link è valido per 1 ora. Se non sei stato tu, ignora questa email.",
		template.HTMLEscapeString(username))
	html = layout("Reimposta la password", body, "Reimposta password", link,
		"Per la tua sicurezza, il link scade tra un'ora e può essere usato una sola volta.")
	return
}

// Notification compone l'email che rispecchia una notifica in-app.
// Ritorna oggetto e corpo vuoti se il tipo non prevede invio email.
func Notification(ntype string, recipient string, data map[string]string) (subject, html string) {
	switch ntype {
	case "new_follower":
		who := data["follower_name"]
		subject = fmt.Sprintf("@%s ha iniziato a seguirti", who)
		body := fmt.Sprintf("<strong>@%s</strong> ha aggiunto il tuo nodo alla propria rete di conoscenze.",
			template.HTMLEscapeString(who))
		html = layout("Nuovo follower", body, "Vedi profilo", AppURL()+"/@"+sanitizeHandle(who), "")
	case "curious_request":
		who := data["requester_name"]
		subject = fmt.Sprintf("@%s è curioso di sapere cosa pensi", who)
		body := fmt.Sprintf("<strong>@%s</strong> vorrebbe leggere il pensiero diretto. "+
			"Puoi accettare o rifiutare dalle notifiche.", template.HTMLEscapeString(who))
		html = layout("Richiesta di curiosità", body, "Apri le notifiche", AppURL()+"/notifications", "")
	case "curious_accepted":
		who := data["accepted_by"]
		subject = fmt.Sprintf("@%s ha accettato la tua richiesta", who)
		body := fmt.Sprintf("<strong>@%s</strong> ha accettato: ora puoi vedere il pensiero diretto.",
			template.HTMLEscapeString(who))
		html = layout("Richiesta accettata", body, "Apri le notifiche", AppURL()+"/notifications", "")
	case "new_comment":
		who := data["commenter_name"]
		subject = fmt.Sprintf("@%s ha commentato un tuo pensiero", who)
		body := fmt.Sprintf("<strong>@%s</strong> ha lasciato un commento sotto un tuo pensiero.",
			template.HTMLEscapeString(who))
		html = layout("Nuovo commento", body, "Apri il profilo", AppURL()+"/@"+sanitizeHandle(who), "")
	}
	return
}

func sanitizeHandle(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "@")
}
