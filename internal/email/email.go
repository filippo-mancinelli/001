// Package email invia email transazionali tramite l'API Resend.
//
// Configurazione (variabili d'ambiente):
//
//	RESEND_API_KEY  chiave API Resend. Se assente, le email vengono
//	                registrate sul log invece di essere inviate (modalità sviluppo).
//	EMAIL_FROM      mittente, es. "pensieri <no-reply@tuodominio.it>".
//	                Default: "pensieri <onboarding@resend.dev>".
//	APP_URL         URL base usato nei link delle email. Default http://localhost:8080.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

type payload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func from() string {
	if v := os.Getenv("EMAIL_FROM"); v != "" {
		return v
	}
	return "pensieri <onboarding@resend.dev>"
}

// AppURL restituisce l'URL base dell'app, senza slash finale.
func AppURL() string {
	if v := os.Getenv("APP_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8080"
}

// Configured indica se è presente una chiave API Resend.
func Configured() bool { return os.Getenv("RESEND_API_KEY") != "" }

// Send invia un'email. Se RESEND_API_KEY non è impostata, il messaggio viene
// loggato (modalità sviluppo) e la funzione ritorna nil senza errori.
func Send(to, subject, html string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("[email:dev] to=%s subject=%q (RESEND_API_KEY assente, invio simulato)", to, subject)
		return nil
	}

	body, err := json.Marshal(payload{
		From:    from(),
		To:      []string{to},
		Subject: subject,
		HTML:    html,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// SendAsync invia l'email in background senza bloccare il chiamante.
// Eventuali errori vengono solo registrati sul log.
func SendAsync(to, subject, html string) {
	go func() {
		if err := Send(to, subject, html); err != nil {
			log.Printf("[email] invio fallito to=%s subject=%q: %v", to, subject, err)
		}
	}()
}
