package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/Cortalo/digitalgarden-backend/internal/config"
	authhandler "github.com/Cortalo/digitalgarden-backend/internal/handler/auth"
	notehandler "github.com/Cortalo/digitalgarden-backend/internal/handler/note"
	"github.com/Cortalo/digitalgarden-backend/internal/infra/authtoken"
	"github.com/Cortalo/digitalgarden-backend/internal/infra/googleauth"
	"github.com/Cortalo/digitalgarden-backend/internal/infra/postgres"
	authservice "github.com/Cortalo/digitalgarden-backend/internal/service/auth"
	noteservice "github.com/Cortalo/digitalgarden-backend/internal/service/note"
	userservice "github.com/Cortalo/digitalgarden-backend/internal/service/user"
)

func main() {
	// No-op in production (Vercel injects env vars directly); fills in
	// DATABASE_URL etc. for local runs.
	_ = godotenv.Load(".env.local")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.AllowedOrigin},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "digitalgarden-backend"})
	})

	noteService := noteservice.NewService(db)
	noteHandler := notehandler.NewHandler(noteService)
	r.GET("/api/notes", noteHandler.List)
	r.GET("/api/notes/:slug", noteHandler.Get)

	googleVerifier := googleauth.NewVerifier(cfg.GoogleClientID)
	tokenIssuer := authtoken.NewIssuer(cfg.JWTSecret)
	userService := userservice.NewService(db)
	authService := authservice.NewService(googleVerifier, userService, tokenIssuer)
	authHandler := authhandler.NewHandler(authService)
	r.POST("/api/auth/google", authHandler.Login)

	// Vercel's Go runtime assigns a port dynamically and proxies to it via
	// the PORT env var — a hardcoded ":8080" is unreachable in that
	// environment. cfg.Port falls back to 8080 for local `go run .`.
	r.Run(":" + cfg.Port)
}
