package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL    string
	Port           string
	AllowedOrigin  string
	GoogleClientID string
	JWTSecret      string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		return Config{}, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}

	return Config{
		DatabaseURL:    dbURL,
		Port:           port,
		AllowedOrigin:  allowedOrigin,
		GoogleClientID: googleClientID,
		JWTSecret:      jwtSecret,
	}, nil
}
