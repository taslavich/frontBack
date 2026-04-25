package config

import (
	"context"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTP       HTTPConfig
	Postgres   PostgresConfig
	JWT        JWTConfig
	ClickHouse ClickHouseConfig
	S3         S3Config
}

type HTTPConfig struct {
	Host string `env:"HTTP_HOSTNAME" env-default:"0.0.0.0"`
	Port int    `env:"HTTP_PORT" env-default:"8080"`
}

type PostgresConfig struct {
	DSN string `env:"POSTGRES_DSN" env-default:"postgres://twinbid:twinbid@localhost:5432/twinbid?sslmode=disable"`
}

type JWTConfig struct {
	Secret     string        `env:"JWT_SECRET" env-required:"true"`
	AccessTTL  time.Duration `env:"ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshTTL time.Duration `env:"REFRESH_TOKEN_TTL" env-default:"720h"`
}

type ClickHouseConfig struct {
	Addr     string `env:"CLICKHOUSE_ADDR" env-default:"localhost:9000"`
	Database string `env:"CLICKHOUSE_DATABASE" env-default:"twinbid"`
	Username string `env:"CLICKHOUSE_USERNAME" env-default:"default"`
	Password string `env:"CLICKHOUSE_PASSWORD" env-default:""`
	Secure   bool   `env:"CLICKHOUSE_SECURE" env-default:"false"`
	Table    string `env:"CLICKHOUSE_STATS_TABLE" env-default:"campaign_stats"`
}

type S3Config struct {
	Endpoint     string        `env:"S3_ENDPOINT" env-default:"http://localhost:9002"`
	Region       string        `env:"AWS_REGION" env-default:"us-east-1"`
	Bucket       string        `env:"S3_BUCKET" env-default:"twinbid-creatives"`
	AccessKey    string        `env:"AWS_ACCESS_KEY_ID" env-default:"minioadmin"`
	SecretKey    string        `env:"AWS_SECRET_ACCESS_KEY" env-default:"minioadmin"`
	UsePathStyle bool          `env:"S3_USE_PATH_STYLE" env-default:"true"`
	PresignTTL   time.Duration `env:"S3_PRESIGN_TTL" env-default:"15m"`
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
