package middleware

import (
	"context"
	"net/http"
	"pensieri/internal/db"
	"pensieri/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			// nessuna sessione: è un visitatore non autenticato che ha provato
			// a usare una funzione riservata -> invito a registrarsi.
			redirectGuest(c, "/register")
			return
		}

		var user models.User
		row := db.Pool.QueryRow(context.Background(), `
			SELECT `+models.UserColumns("u")+`
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now())
		if err = models.ScanUser(row, &user); err != nil {
			// sessione scaduta o non valida: torna al login.
			c.SetCookie("session_token", "", -1, "/", "", false, true)
			redirectGuest(c, "/login")
			return
		}

		touchPresence(user.ID)
		c.Set("user", user)
		c.Next()
	}
}

// touchPresence registra l'attività dell'utente aggiornando il suo ultimo battito
// (last_seen): è ciò che lo mantiene "online". Viene chiamata a ogni richiesta
// autenticata (navigazione e heartbeat periodico del client); quando l'app/tab si
// chiude i battiti cessano e l'utente torna automaticamente offline. È sincrona —
// un UPDATE per chiave primaria è trascurabile — così l'ordine con eventuali altre
// scritture nella stessa richiesta (es. l'azzeramento al logout) è deterministico.
func touchPresence(userID string) {
	db.Pool.Exec(context.Background(),
		`UPDATE users SET last_seen = NOW() WHERE id = $1`, userID)
}

// redirectGuest interrompe la richiesta indirizzando l'utente a dest. Per le
// richieste HTMX usa l'header HX-Redirect (HTMX naviga l'intera pagina),
// altrimenti un classico redirect 302.
func redirectGuest(c *gin.Context, dest string) {
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", dest)
		c.Status(http.StatusOK)
	} else {
		c.Redirect(http.StatusFound, dest)
	}
	c.Abort()
}

// AdminOnly va montato DOPO Auth(): richiede una sessione valida il cui utente
// abbia is_admin = true, altrimenti risponde 404 per non rivelare l'esistenza
// delle rotte amministrative (la pagina admin è "segreta": non linkata e
// indistinguibile da un percorso inesistente per chi non è admin).
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("user")
		if u, isUser := v.(models.User); !ok || !isUser || !u.IsAdmin {
			c.Status(http.StatusNotFound)
			c.Abort()
			return
		}
		c.Next()
	}
}

func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			c.Next()
			return
		}

		var user models.User
		row := db.Pool.QueryRow(context.Background(), `
			SELECT `+models.UserColumns("u")+`
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > $2
		`, token, time.Now())
		if err = models.ScanUser(row, &user); err == nil {
			touchPresence(user.ID)
			c.Set("user", user)
		}
		c.Next()
	}
}
