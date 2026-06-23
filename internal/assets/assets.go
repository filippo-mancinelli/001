// Package assets gestisce il versionamento (cache-busting) degli asset statici.
//
// Gli URL degli asset vengono "firmati" con un hash del contenuto (?v=...).
// Quando il contenuto di un asset cambia, cambia anche il suo URL: il browser
// (e il service worker) non lo hanno mai visto e lo riscaricano sempre. Questo
// rende gli asset di fatto immutabili e cacheabili in modo aggressivo, senza
// rischiare di servire CSS/JS stantii dopo un deploy.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
)

// hashedFiles sono i file il cui contenuto determina la versione degli asset.
// Sono quelli che cambiano tra un deploy e l'altro e che, se serviti stantii,
// rompono il frontend (CSS) o il comportamento (JS).
var hashedFiles = []string{
	"web/static/css/retro.css",
	"web/static/js/htmx.min.js",
}

var (
	once    sync.Once
	version string
)

// Version restituisce un token breve e stabile derivato dal contenuto degli
// asset. Cambia se e solo se cambia il contenuto di uno degli hashedFiles.
func Version() string {
	once.Do(func() {
		h := sha256.New()
		for _, f := range hashedFiles {
			b, err := os.ReadFile(f)
			if err != nil {
				// In caso di errore di lettura non vogliamo bloccare l'avvio:
				// includiamo il nome del file così la versione resta deterministica.
				h.Write([]byte(f))
				continue
			}
			h.Write(b)
		}
		version = hex.EncodeToString(h.Sum(nil))[:12]
	})
	return version
}

// URL appende il token di versione al path di un asset statico.
func URL(path string) string {
	return path + "?v=" + Version()
}
