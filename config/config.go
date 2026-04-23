package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	PostgresDSN          string
	JWTSecret            string
	AccessTokenTTL       int // minutes
	RefreshTokenTTL      int // hours
	FrontendURL          string
	S3Region             string
	S3Bucket             string
	S3AccessKey          string
	S3SecretKey          string
	S3Endpoint           string
	S3UsePathStyle       bool
	S3UploadTTLSeconds   int
	S3DownloadTTLSeconds int
}

func Load() *Config {
	return &Config{
		Port:                 getEnv("PORT", "8080"),
		PostgresDSN:          getEnv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/twinbid?sslmode=disable"),
		JWTSecret:            getEnv("JWT_SECRET", "supersecretkey"),
		AccessTokenTTL:       getEnvInt("ACCESS_TOKEN_TTL", 15),
		RefreshTokenTTL:      getEnvInt("REFRESH_TOKEN_TTL", 720),
		FrontendURL:          getEnv("FRONTEND_URL", "https://twinbid.io"),
		S3Region:             getEnv("S3_REGION", "us-east-1"),
		S3Bucket:             getEnv("S3_BUCKET", ""),
		S3AccessKey:          getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:          getEnv("S3_SECRET_KEY", ""),
		S3Endpoint:           getEnv("S3_ENDPOINT", ""),
		S3UsePathStyle:       getEnvBool("S3_USE_PATH_STYLE", false),
		S3UploadTTLSeconds:   getEnvInt("S3_UPLOAD_TTL_SECONDS", 900),
		S3DownloadTTLSeconds: getEnvInt("S3_DOWNLOAD_TTL_SECONDS", 900),
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

func getEnvBool(key string, def bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return def
}
