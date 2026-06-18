package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetManifest serve il web app manifest dalla radice del sito, così che lo
// scope della PWA sia "/" e non "/static/".
func GetManifest(c *gin.Context) {
	c.Header("Content-Type", "application/manifest+json; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.File("web/static/manifest.webmanifest")
}

// GetServiceWorker serve il service worker dalla radice, in modo che possa
// controllare l'intero sito (scope "/").
func GetServiceWorker(c *gin.Context) {
	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.Header("Service-Worker-Allowed", "/")
	// niente cache aggressiva: vogliamo poter aggiornare il SW rapidamente
	c.Header("Cache-Control", "no-cache")
	c.File("web/static/sw.js")
}

// GetInstall mostra la pagina che guida (o avvia) l'installazione della PWA.
// È pubblica: chiunque deve poterla raggiungere e installare l'app.
func GetInstall(c *gin.Context) {
	c.HTML(http.StatusOK, "install.html", gin.H{
		"Host": c.Request.Host,
	})
}
