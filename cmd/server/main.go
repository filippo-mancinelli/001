package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"pensieri/internal/assets"
	"pensieri/internal/db"
	"pensieri/internal/handlers"
	"pensieri/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// templateFuncs espone helper usati nei template (avatar/iniziali, ecc.).
var templateFuncs = template.FuncMap{
	// initial restituisce la prima lettera maiuscola, per gli avatar fallback.
	"initial": func(s string) string {
		r := []rune(strings.TrimSpace(s))
		if len(r) == 0 {
			return "?"
		}
		return strings.ToUpper(string(r[0]))
	},
	// avatarColor deriva un colore stabile dallo username (avatar generato).
	// Ritorna template.CSS perché il valore finisce in un attributo style: senza
	// il tipo CSS il sanitizer di html/template lo sostituirebbe con "ZgotmplZ",
	// lasciando gli avatar fallback bianchi. Il colore deriva da un hash dello
	// username (nessun input arbitrario), quindi è sicuro marcarlo come CSS.
	"avatarColor": func(s string) template.CSS {
		var h int32
		for _, r := range s {
			h = h*31 + r
		}
		hue := ((h % 360) + 360) % 360
		return template.CSS(fmt.Sprintf("hsl(%d, 55%%, 52%%)", hue))
	},
	// identicon genera un SVG identicon 5×5 simmetrico (stile pixel-art retrò)
	// derivato deterministicamente dallo username. Sicuro: lo username non compare
	// nell'SVG come testo, viene solo usato come seed per il generatore.
	"identicon": func(username string) template.HTML {
		var h uint32
		for _, r := range username {
			h = h*31 + uint32(r)
		}
		hue := int(h % 360)
		fg := fmt.Sprintf("hsl(%d,58%%,38%%)", hue)
		bg := fmt.Sprintf("hsl(%d,35%%,93%%)", hue)

		const (
			size     = 5
			cellSize = 8
			svgSize  = size * cellSize
		)

		pixels := make([]bool, size*size)
		seed := h
		for row := 0; row < size; row++ {
			for col := 0; col < 3; col++ {
				seed = seed*1664525 + 1013904223
				on := (seed>>27)&1 == 1
				pixels[row*size+col] = on
				if col < 2 {
					pixels[row*size+(4-col)] = on // simmetria orizzontale
				}
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, `<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">`,
			svgSize, svgSize, svgSize, svgSize)
		fmt.Fprintf(&sb, `<rect width="%d" height="%d" fill="%s"/>`, svgSize, svgSize, bg)
		for i, on := range pixels {
			if on {
				r := i / size
				c := i % size
				fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
					c*cellSize, r*cellSize, cellSize, cellSize, fg)
			}
		}
		sb.WriteString(`</svg>`)
		return template.HTML(sb.String())
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	// firma l'URL di un asset statico con la versione del contenuto (?v=hash)
	"asset": assets.URL,
}

// staticCacheHeaders: URL versionati immutabili (1 anno), gli altri cache breve.
func staticCacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query("v") != "" {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=300")
		}
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}

	if err := db.Connect(); err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Pool.Close()

	if err := runMigrations(); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	r := gin.Default()
	// asset statici: cache lunga per gli URL versionati, breve per gli altri
	staticGroup := r.Group("/static", staticCacheHeaders())
	staticGroup.Static("/", "web/static")

	// PWA: manifest e service worker serviti dalla radice (scope "/"),
	// pagina di installazione pubblica.
	r.GET("/manifest.webmanifest", handlers.GetManifest)
	r.GET("/sw.js", handlers.GetServiceWorker)
	r.GET("/install", handlers.GetInstall)

	tmpl := template.Must(template.New("").Funcs(templateFuncs).ParseGlob("web/templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/login", handlers.GetLogin)
	r.POST("/login", handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", handlers.PostRegister)
	r.GET("/auth/google", handlers.GetGoogleLogin)
	r.GET("/auth/google/callback", handlers.GetGoogleCallback)
	r.GET("/forgot", handlers.GetForgot)
	r.POST("/forgot", handlers.PostForgot)
	r.GET("/reset", handlers.GetReset)
	r.POST("/reset", handlers.PostReset)

	auth := r.Group("/", middleware.Auth())
	{
		auth.POST("/logout", handlers.PostLogout)
		auth.GET("/", handlers.GetFeed)
		auth.GET("/@:username", handlers.GetProfile)
		auth.GET("/settings", handlers.GetSettings)
		auth.POST("/settings/profile", handlers.PostProfileSettings)
		auth.POST("/settings/account", handlers.PostAccountSettings)
		auth.POST("/settings/password", handlers.PostPasswordSettings)
		auth.GET("/search", handlers.GetSearch)
		auth.GET("/search/results", handlers.GetSearchResults)
		auth.POST("/follow/:username", handlers.PostFollow)
		auth.DELETE("/follow/:username", handlers.DeleteFollow)
		auth.GET("/notifications", handlers.GetNotifications)
		auth.GET("/notifications/count", handlers.GetNotificationsCount)
		auth.GET("/network", handlers.GetNetwork)

		auth.POST("/pensieri", handlers.PostPensiero)
		auth.GET("/pensieri/nuovo", handlers.GetEditorPensiero)
		auth.GET("/pensieri/annulla", handlers.AnnullaEditorPensiero)
		auth.GET("/pensieri/:id/modifica", handlers.GetEditorPensiero)
		auth.POST("/pensieri/:id/versioni", handlers.PostVersionePensiero)
		auth.DELETE("/pensieri/:id/versioni/:audienceID", handlers.EliminaVersionePensiero)

		auth.GET("/pensieri/:id/commenti", handlers.GetCommenti)
		auth.GET("/pensieri/:id/commenti/chiudi", handlers.GetCommentiChiudi)
		auth.POST("/pensieri/:id/commenti", handlers.PostCommento)
		auth.DELETE("/commenti/:id", handlers.DeleteCommento)

		auth.GET("/users/:username/mini", handlers.GetUserMini)

		auth.POST("/dna/:id", handlers.PostDna)

		auth.POST("/curious/:id", handlers.PostCurious)
		auth.POST("/curious/:id/accept", handlers.PostCuriousAccept)
		auth.POST("/curious/:id/reject", handlers.PostCuriousReject)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("server on :%s", port)
	r.Run(":" + port)
}

func runMigrations() error {
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		var applied bool
		if err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, base,
		).Scan(&applied); err != nil {
			return fmt.Errorf("check %s: %w", base, err)
		}
		if applied {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, base,
		); err != nil {
			return fmt.Errorf("record %s: %w", base, err)
		}
		log.Printf("migration applied: %s", base)
	}
	return nil
}
