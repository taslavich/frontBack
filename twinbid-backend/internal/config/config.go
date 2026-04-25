package config

import (
	"context"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTP        HTTPConfig       `env:"HTTP"`
	Postgres    PostgresConfig   `env:"POSTGRES"`
	JWT         JWTConfig        `env:"JWT"`
	ClickHouse  ClickHouseConfig `env:"CLICKHOUSE"`
	S3          S3Config         `env:"S3"`
	TLSCertFile string           `env:"TLS_CERT_FILE"`
	TLSKeyFile  string           `env:"TLS_KEY_FILE"`
}

type HTTPConfig struct {
	Host string `env:"HOST" env-default:"0.0.0.0"`
	Port int    `env:"PORT" env-default:"8080"`
}

type PostgresConfig struct {
	DSN string `env:"DSN" env-default:"postgres://twinbid:twinbid@localhost:5432/twinbid?sslmode=disable"`
}

type JWTConfig struct {
	Secret     string        `env:"SECRET" env-required:"true"`
	AccessTTL  time.Duration `env:"ACCESS_TTL" env-default:"15m"`
	RefreshTTL time.Duration `env:"REFRESH_TTL" env-default:"720h"`
}

type ClickHouseConfig struct {
	Addr     string `env:"ADDR" env-default:"localhost:9000"`
	Database string `env:"DATABASE" env-default:"twinbid"`
	Username string `env:"USERNAME" env-default:"default"`
	Password string `env:"PASSWORD" env-default:""`
	Secure   bool   `env:"SECURE" env-default:"false"`
	Table    string `env:"STATS_TABLE" env-default:"campaign_stats"`
}

type S3Config struct {
	Endpoint     string        `env:"ENDPOINT" env-default:"http://localhost:9002"`
	Region       string        `env:"REGION" env-default:"us-east-1"`
	Bucket       string        `env:"BUCKET" env-default:"twinbid-creatives"`
	AccessKey    string        `env:"ACCESS_KEY" env-default:"minioadmin"`
	SecretKey    string        `env:"SECRET_KEY" env-default:"minioadmin"`
	UsePathStyle bool          `env:"USE_PATH_STYLE" env-default:"true"`
	PresignTTL   time.Duration `env:"PRESIGN_TTL" env-default:"15m"`
}

func getEnvFileNames() []string {
	return []string{".env.local", ".env", "api.env"}
}

func Load(ctx context.Context) (*Config, error) {
	for _, fileName := range getEnvFileNames() {
		if err := godotenv.Load(fileName); err != nil {
			log.Printf("error loading %s: %v", fileName, err)
		}
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
