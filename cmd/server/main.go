package main

import (
	"context"
	"html/template"
	"log"
	"os"
	"thoughts/internal/db"
	"thoughts/internal/handlers"
	"thoughts/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

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

	tmpl := template.Must(template.New("").ParseGlob("web/templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/login", handlers.GetLogin)
	r.POST("/login", handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", handlers.PostRegister)

	auth := r.Group("/", middleware.Auth())
	{
		auth.POST("/logout", handlers.PostLogout)
		auth.GET("/", handlers.GetFeed)
		auth.GET("/@:username", handlers.GetProfile)
		auth.POST("/follow/:username", handlers.PostFollow)
		auth.DELETE("/follow/:username", handlers.DeleteFollow)
		auth.GET("/notifications", handlers.GetNotifications)

		auth.POST("/thoughts", handlers.PostThought)
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
	sql, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(context.Background(), string(sql))
	return err
}
