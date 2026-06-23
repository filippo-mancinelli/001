// Package assets versiona gli asset statici (cache-busting via ?v=hash).
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
)

// file il cui contenuto determina la versione degli asset
var hashedFiles = []string{
	"web/static/css/retro.css",
	"web/static/js/htmx.min.js",
}

var (
	once    sync.Once
	version string
)

// Version è un token derivato dal contenuto degli asset; cambia solo se cambiano.
func Version() string {
	once.Do(func() {
		h := sha256.New()
		for _, f := range hashedFiles {
			b, err := os.ReadFile(f)
			if err != nil {
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
