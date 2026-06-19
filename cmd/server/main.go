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
	"avatarColor": func(s string) string {
		var h int32
		for _, r := range s {
			h = h*31 + r
		}
		hue := ((h % 360) + 360) % 360
		return fmt.Sprintf("hsl(%d, 55%%, 52%%)", hue)
	},
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
	r.Static("/static", "web/static")

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
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Pool.Exec(context.Background(), string(sql)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		log.Printf("migration applied: %s", filepath.Base(f))
	}
	return nil
}
