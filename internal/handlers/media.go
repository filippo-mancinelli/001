package handlers

import (
	"io"
	"net/http"
	"regexp"

	"pensieri/internal/storage"

	"github.com/gin-gonic/gin"
)

// safeMediaName limita i nomi file servibili a quelli generati da noi
// (uuid + estensione), evitando path traversal o accesso a chiavi arbitrarie.
var safeMediaName = regexp.MustCompile(`^[A-Za-z0-9_-]+\.(jpg|jpeg|png|gif|webp)$`)

func serveMedia(c *gin.Context, prefix string) {
	name := c.Param("name")
	if !safeMediaName.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}

	obj, err := storage.Download(c.Request.Context(), prefix+"/"+name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer obj.Body.Close()

	ct := obj.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Status(http.StatusOK)
	io.Copy(c.Writer, obj.Body)
}

// GetAvatarMedia fa da proxy verso il bucket S3 per le immagini avatar caricate.
// Il bucket resta privato: i file vengono letti dall'app (con le credenziali
// IAM) e ritrasmessi al browser. Rotta pubblica perché gli avatar sono visibili
// anche nei profili/feed pubblici. Cache lunga: il nome file è univoco per
// upload, quindi il contenuto a una data chiave non cambia mai.
func GetAvatarMedia(c *gin.Context) {
	serveMedia(c, "avatars")
}

// GetPensieroMedia fa da proxy verso il bucket S3 per le immagini allegate ai pensieri.
func GetPensieroMedia(c *gin.Context) {
	serveMedia(c, "pensieri")
}
