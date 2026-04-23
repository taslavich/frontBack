package config

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Кастомный тип для map[string]string
type MapStringToString map[string]string

func (m *MapStringToString) SetValue(value string) error {
	*m = make(MapStringToString)
	if value == "" {
		return nil
	}

	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		idx := strings.Index(pair, "|")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(pair[:idx])
		val := strings.TrimSpace(pair[idx+1:])
		(*m)[key] = val
	}
	return nil
}

// Кастомный тип для []string
type ListString []string

func (l *ListString) SetValue(value string) error {
	*l = make(ListString, 0)
	if value == "" {
		return nil
	}
	items := strings.Split(value, ",")
	for _, item := range items {
		*l = append(*l, strings.TrimSpace(item))
	}
	return nil
}

// Основная конфигурация для API бэкенда (auth + profile)
type ApiConfig struct {
	HttpServer      HttpServer
	PostgresDSN     string `yaml:"POSTGRES_DSN" env:"POSTGRES_DSN" env-default:"postgres://user:pass@localhost:5432/twinbid?sslmode=disable"`
	JWTSecret       string `yaml:"JWT_SECRET" env:"JWT_SECRET" env-default:"supersecretkey"`
	AccessTokenTTL  int    `yaml:"ACCESS_TOKEN_TTL" env:"ACCESS_TOKEN_TTL" env-default:"15"`    // minutes
	RefreshTokenTTL int    `yaml:"REFRESH_TOKEN_TTL" env:"REFRESH_TOKEN_TTL" env-default:"720"` // hours
	FrontendURL     string `yaml:"FRONTEND_URL" env:"FRONTEND_URL" env-default:"https://twinbid.io"`
}

type HttpServer struct {
	Host string `yaml:"HTTP_HOSTNAME" env:"HTTP_HOSTNAME" env-default:"0.0.0.0"`
	Port uint16 `yaml:"HTTP_PORT" env:"HTTP_PORT" env-default:"8080"`
}

func (s HttpServer) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func getEnvFileNames() []string {
	return []string{".env.local", ".env", "api.env"}
}

func LoadConfig[
	T ApiConfig,
](ctx context.Context) (*T, error) {
	for _, fileName := range getEnvFileNames() {
		if err := godotenv.Load(fileName); err != nil {
			log.Printf("error loading %s file: %v", fileName, err)
		}
	}

	var cfg T
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
