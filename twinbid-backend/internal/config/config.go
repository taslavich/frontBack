package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP       HTTPConfig
	Postgres   PostgresConfig
	JWT        JWTConfig
	ClickHouse ClickHouseConfig
	S3         S3Config

	TLSCertFile string
	TLSKeyFile  string
}

type HTTPConfig struct {
	Host string
	Port int
}

type PostgresConfig struct {
	DSN string
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type ClickHouseConfig struct {
	Addr     string
	Database string
	Username string
	Password string
	Secure   bool
	Table    string
}

type S3Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	PresignTTL   time.Duration
}

func Load() (Config, error) {
	port, err := intEnv("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	accessTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := durationEnv("REFRESH_TOKEN_TTL", 720*time.Hour)
	if err != nil {
		return Config{}, err
	}
	presignTTL, err := durationEnv("S3_PRESIGN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	secure, err := boolEnv("CLICKHOUSE_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	usePathStyle, err := boolEnv("S3_USE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Host: strEnv("HTTP_HOST", "0.0.0.0"),
			Port: port,
		},
		Postgres: PostgresConfig{
			DSN: strEnv("POSTGRES_DSN", "postgres://twinbid:twinbid@localhost:5432/twinbid?sslmode=disable"),
		},
		JWT: JWTConfig{
			Secret:     strEnv("JWT_SECRET", "change-me"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		ClickHouse: ClickHouseConfig{
			Addr:     strEnv("CLICKHOUSE_ADDR", "localhost:9000"),
			Database: strEnv("CLICKHOUSE_DATABASE", "twinbid"),
			Username: strEnv("CLICKHOUSE_USERNAME", "default"),
			Password: strEnv("CLICKHOUSE_PASSWORD", ""),
			Secure:   secure,
			Table:    strEnv("CLICKHOUSE_STATS_TABLE", "campaign_stats"),
		},
		S3: S3Config{
			Endpoint:     strEnv("S3_ENDPOINT", "http://localhost:9002"),
			Region:       strEnv("S3_REGION", "us-east-1"),
			Bucket:       strEnv("S3_BUCKET", "twinbid-creatives"),
			AccessKey:    strEnv("S3_ACCESS_KEY", "minioadmin"),
			SecretKey:    strEnv("S3_SECRET_KEY", "minioadmin"),
			UsePathStyle: usePathStyle,
			PresignTTL:   presignTTL,
		},
	}
	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func strEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func intEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	out, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return out, nil
}

func boolEnv(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	out, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return out, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	out, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return out, nil
}
