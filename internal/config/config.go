package config

import (
	"log"
	"os"
)

type Config struct {
	PORT                  string
	DB_URL                string
	JWT_SECRET            string
	ENCRYPTION_KEY        string
	APP_BASE_URL          string
	UPSTREAM_PROVIDER_URL string
	Address               string
}

func Load() Config {
	port := getEnvOrFatal("PORT")
	dbURL := getEnvOrFatal("DB_URL")
	jwtSecret := getEnvOrFatal("JWT_SECRET")
	encryptionKey := getEnvOrFatal("ENCRYPTION_KEY")
	appBaseURL := getEnvOrFatal("APP_BASE_URL")
	upstreamURL := getEnvOrFatal("UPSTREAM_PROVIDER_URL")

	return Config{
		PORT:                  port,
		DB_URL:                dbURL,
		JWT_SECRET:            jwtSecret,
		ENCRYPTION_KEY:        encryptionKey,
		APP_BASE_URL:          appBaseURL,
		UPSTREAM_PROVIDER_URL: upstreamURL,
		Address:               ":" + port,
	}
}

func getEnvOrFatal(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return val
}