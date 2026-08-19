package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	CORSOrigin  string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		CORSOrigin:  getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}
	if c.DatabaseURL == "" {
		c.DatabaseURL = "postgres://postgres:postgres@localhost:5432/kanban_dev?sslmode=disable"
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "dev-secret-change-in-production"
	}
	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
