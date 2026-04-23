package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	PostgresDSN     string
	JWTSecret       string
	AccessTokenTTL  int // minutes
	RefreshTokenTTL int // hours
	FrontendURL     string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		PostgresDSN:     getEnv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/twinbid?sslmode=disable"),
		JWTSecret:       getEnv("JWT_SECRET", "supersecretkey"),
		AccessTokenTTL:  getEnvInt("ACCESS_TOKEN_TTL", 15),
		RefreshTokenTTL: getEnvInt("REFRESH_TOKEN_TTL", 720),
		FrontendURL:     getEnv("FRONTEND_URL", "https://twinbid.io"),
	}
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}
