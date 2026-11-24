package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	PORT                 string
	DB_URL               string
	JWT_SECRET           string
	ENCRYPTION_KEY       string
	APP_BASE_URL         string
	CORS_ALLOWED_ORIGINS []string
	Address              string
}

func Load() Config {
	port := getEnvOrFatal("PORT")
	dbURL := getEnvOrFatal("DB_URL")
	jwtSecret := getEnvOrFatal("JWT_SECRET")
	encryptionKey := getEnvOrFatal("ENCRYPTION_KEY")
	appBaseURL := getEnvOrFatal("APP_BASE_URL")
	corsOrigins := getEnvOrFatal("CORS_ALLOWED_ORIGINS")

	return Config{
		PORT:                 port,
		DB_URL:               dbURL,
		JWT_SECRET:           jwtSecret,
		ENCRYPTION_KEY:       encryptionKey,
		APP_BASE_URL:         appBaseURL,
		CORS_ALLOWED_ORIGINS: splitAndTrim(corsOrigins),
		Address:              ":" + port,
	}
}

func getEnvOrFatal(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return val
}

// splitAndTrim parses a comma-separated environment value into a clean list
func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
