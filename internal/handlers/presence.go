package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PostPresencePing è l'heartbeat del client: viene chiamato periodicamente dal
// browser finché l'app/tab resta aperta. L'aggiornamento effettivo di last_seen
// avviene nel middleware Auth (che intercetta ogni richiesta autenticata), quindi
// qui basta rispondere senza corpo. Quando la tab si chiude i battiti cessano e
// l'utente torna offline automaticamente allo scadere della finestra di presenza.
func PostPresencePing(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
