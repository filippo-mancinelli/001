package handlers

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"pensieri/internal/assets"

	"github.com/gin-gonic/gin"
)

// GetManifest serve il web app manifest dalla radice del sito, così che lo
// scope della PWA sia "/" e non "/static/".
func GetManifest(c *gin.Context) {
	c.Header("Content-Type", "application/manifest+json; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.File("web/static/manifest.webmanifest")
}

var (
	swOnce sync.Once
	swBody string
)

// loadServiceWorker legge sw.js una volta e inietta la versione degli asset.
func loadServiceWorker() string {
	swOnce.Do(func() {
		b, err := os.ReadFile("web/static/sw.js")
		if err != nil {
			swBody = ""
			return
		}
		swBody = strings.ReplaceAll(string(b), "__ASSET_VERSION__", assets.Version())
	})
	return swBody
}

// GetServiceWorker serve il service worker dalla radice (scope "/").
func GetServiceWorker(c *gin.Context) {
	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.Header("Service-Worker-Allowed", "/")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, loadServiceWorker())
}

// GetInstall mostra la pagina che guida (o avvia) l'installazione della PWA.
// È pubblica: chiunque deve poterla raggiungere e installare l'app.
func GetInstall(c *gin.Context) {
	c.HTML(http.StatusOK, "install.html", gin.H{
		"Host": c.Request.Host,
	})
}
