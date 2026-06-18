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
	"thoughts/internal/db"
	"thoughts/internal/handlers"
	"thoughts/internal/middleware"

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
		auth.POST("/follow/:username", handlers.PostFollow)
		auth.DELETE("/follow/:username", handlers.DeleteFollow)
		auth.GET("/notifications", handlers.GetNotifications)

		auth.POST("/thoughts", handlers.PostThought)
		auth.GET("/thoughts/new", handlers.GetThoughtEditor)
		auth.GET("/thoughts/cancel", handlers.CancelThoughtEditor)
		auth.GET("/thoughts/:id/edit", handlers.GetThoughtEditor)
		auth.POST("/thoughts/:id/versions", handlers.PostThoughtVersion)
		auth.DELETE("/thoughts/:id/versions/:audienceID", handlers.DeleteThoughtVersion)

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
