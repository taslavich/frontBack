package config

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTP             HTTPConfig
	Postgres         PostgresConfig
	JWT              JWTConfig
	SMTP             SMTPConfig
	ClickHouse       ClickHouseConfig
	S3               S3Config
	Notifications    NotificationsConfig
	SpendSync        SpendSyncConfig
	PassimPay        PassimPayConfig
	Cryptomus        CryptomusConfig
	Bot              BotConfig
	PublicAPIBaseURL string `env:"PUBLIC_API_BASE_URL" env-default:"http://localhost:8080"`

	// POP
	SspPopAdlFeeds MapStringToString `yaml:"SSP_POP_ADL_FEEDS" env:"SSP_POP_ADL_FEEDS"`
	SspPopMcFeeds  MapStringToString `yaml:"SSP_POP_MC_FEEDS" env:"SSP_POP_MC_FEEDS"`

	// BAN
	SspBanAdlFeeds MapStringToString `yaml:"SSP_BAN_ADL_FEEDS" env:"SSP_BAN_ADL_FEEDS"`
	SspBanMcFeeds  MapStringToString `yaml:"SSP_BAN_MC_FEEDS" env:"SSP_BAN_MC_FEEDS"`

	// NAT
	SspNatAdlFeeds MapStringToString `yaml:"SSP_NAT_ADL_FEEDS" env:"SSP_NAT_ADL_FEEDS"`
	SspNatMcFeeds  MapStringToString `yaml:"SSP_NAT_MC_FEEDS" env:"SSP_NAT_MC_FEEDS"`

	// IPP
	SspIppAdlFeeds MapStringToString `yaml:"SSP_IPP_ADL_FEEDS" env:"SSP_IPP_ADL_FEEDS"`
	SspIppMcFeeds  MapStringToString `yaml:"SSP_IPP_MC_FEEDS" env:"SSP_IPP_MC_FEEDS"`
}

type HTTPConfig struct {
	Host string `env:"HTTP_HOSTNAME" env-default:"0.0.0.0"`
	Port int    `env:"HTTP_PORT" env-default:"8080"`
}

type PostgresConfig struct {
	DSN string `env:"POSTGRES_DSN" env-default:"postgres://twinbid:twinbid@localhost:5432/twinbid?sslmode=disable"`
}

type JWTConfig struct {
	Secret                string        `env:"JWT_SECRET" env-required:"true"`
	AccessTTL             time.Duration `env:"ACCESS_TOKEN_TTL" env-default:"15m"`
	RefreshTTL            time.Duration `env:"REFRESH_TOKEN_TTL" env-default:"720h"`
	RegistrationTokenTTL  time.Duration `env:"REGISTRATION_TOKEN_TTL" env-default:"6h"`
	RegistrationCleanupIn time.Duration `env:"REGISTRATION_TOKEN_CLEANUP_INTERVAL" env-default:"10m"`
}

type SMTPConfig struct {
	Host        string `env:"SMTP_HOST" env-default:"mail.hosting.reg.ru"`
	Port        int    `env:"SMTP_PORT" env-default:"587"`
	Username    string `env:"SMTP_USERNAME" env-default:""`
	Password    string `env:"SMTP_PASSWORD" env-default:""`
	From        string `env:"SMTP_FROM" env-default:"noreply@twinbidex.com"`
	TLSType     string `env:"SMTP_TLS_TYPE" env-default:"starttls"`
	FrontendURL string `env:"FRONTEND_URL" env-default:"https://twinbid.io"`
	VerifyURL   string `env:"VERIFY_BASE_URL" env-default:""`
}

type ClickHouseConfig struct {
	Addr         string `env:"CLICKHOUSE_ADDR" env-default:"localhost:9000"`
	Database     string `env:"CLICKHOUSE_DATABASE" env-default:"ads"`
	Username     string `env:"CLICKHOUSE_USERNAME" env-default:"default"`
	Password     string `env:"CLICKHOUSE_PASSWORD" env-default:""`
	Secure       bool   `env:"CLICKHOUSE_SECURE" env-default:"false"`
	Table        string `env:"CLICKHOUSE_STATS_TABLE" env-default:"agg_stats"`
	TrafficTable string `env:"CLICKHOUSE_TRAFFIC_TABLE" env-default:"traffic_volume_hourly"`
}

type BotConfig struct {
	BaseURL        string `env:"BOT_BASE_URL" env-default:"http://127.0.0.1:8090"`
	InternalSecret string `env:"BOT_INTERNAL_SECRET" env-required:"true"`
	AdminUserID    string `env:"BOT_ADMIN_USER_ID" env-required:"true"`
}
type PassimPayConfig struct {
	BaseURL               string        `env:"PASSIMPAY_BASE_URL" env-default:"https://api.passimpay.io"`
	PlatformID            int64         `env:"PASSIMPAY_PLATFORM_ID" env-default:"0"`
	APIKey                string        `env:"PASSIMPAY_API_KEY" env-default:""`
	CreateInvoicePath     string        `env:"PASSIMPAY_CREATE_INVOICE_PATH" env-default:"/v2/createorder"`
	CheckInvoicePath      string        `env:"PASSIMPAY_CHECK_INVOICE_PATH" env-default:"/v2/orderstatus"`
	InvoiceType           int           `env:"PASSIMPAY_INVOICE_TYPE" env-default:"1"`
	CurrencyIDs           string        `env:"PASSIMPAY_CURRENCY_IDS" env-default:""`
	Timeout               time.Duration `env:"PASSIMPAY_TIMEOUT" env-default:"10s"`
	ReconcileInterval     time.Duration `env:"PASSIMPAY_RECONCILE_INTERVAL" env-default:"5m"`
	ReconcileBatchSize    int           `env:"PASSIMPAY_RECONCILE_BATCH_SIZE" env-default:"20"`
	ReconcileRequestDelay time.Duration `env:"PASSIMPAY_RECONCILE_REQUEST_DELAY" env-default:"150ms"`
	ReconcileRetryDelay   time.Duration `env:"PASSIMPAY_RECONCILE_RETRY_DELAY" env-default:"5m"`
}

type CryptomusConfig struct {
	BaseURL               string        `env:"CRYPTOMUS_BASE_URL" env-default:"https://api.cryptomus.com"`
	MerchantUUID          string        `env:"CRYPTOMUS_MERCHANT_UUID" env-default:""`
	PaymentAPIKey         string        `env:"CRYPTOMUS_PAYMENT_API_KEY" env-default:""`
	CreateInvoicePath     string        `env:"CRYPTOMUS_CREATE_INVOICE_PATH" env-default:"/v1/payment"`
	CheckInvoicePath      string        `env:"CRYPTOMUS_CHECK_INVOICE_PATH" env-default:"/v1/payment/info"`
	WebhookURL            string        `env:"CRYPTOMUS_WEBHOOK_URL" env-default:""`
	SubtractPercent       int           `env:"CRYPTOMUS_SUBTRACT_PERCENT" env-default:"100"`
	Timeout               time.Duration `env:"CRYPTOMUS_TIMEOUT" env-default:"10s"`
	ReconcileInterval     time.Duration `env:"CRYPTOMUS_RECONCILE_INTERVAL" env-default:"5m"`
	ReconcileBatchSize    int           `env:"CRYPTOMUS_RECONCILE_BATCH_SIZE" env-default:"20"`
	ReconcileRequestDelay time.Duration `env:"CRYPTOMUS_RECONCILE_REQUEST_DELAY" env-default:"150ms"`
	ReconcileRetryDelay   time.Duration `env:"CRYPTOMUS_RECONCILE_RETRY_DELAY" env-default:"5m"`
}

type S3Config struct {
	Endpoint     string `env:"S3_ENDPOINT" env-default:"http://127.0.0.1:9000"`
	Region       string `env:"AWS_REGION" env-default:"us-east-1"`
	Bucket       string `env:"S3_BUCKET" env-default:"creatives"`
	AccessKey    string `env:"AWS_ACCESS_KEY_ID" env-default:"minioadmin"`
	SecretKey    string `env:"AWS_SECRET_ACCESS_KEY" env-default:"minioadmin"`
	UsePathStyle bool   `env:"S3_USE_PATH_STYLE" env-default:"true"`
}

// Кастомный тип для map[string]string
type MapStringToString map[string]string

func (m *MapStringToString) SetValue(value string) error {
	*m = make(MapStringToString)
	if value == "" {
		return nil
	}

	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		// Ищем только ПЕРВЫЙ знак | как разделитель ключ-значение
		idx := strings.Index(pair, "|")
		if idx == -1 {
			continue // пропускаем некорректные пары
		}

		key := strings.TrimSpace(pair[:idx])
		valueStr := strings.TrimSpace(pair[idx+1:])
		(*m)[key] = valueStr
	}
	return nil
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

type SpendSyncConfig struct {
	Interval time.Duration `env:"STATS_SPEND_SYNC_INTERVAL" env-default:"1m"`
	Timeout  time.Duration `env:"STATS_SPEND_SYNC_TIMEOUT" env-default:"50s"`
}

type NotificationsConfig struct {
	LowBalanceCheckInterval        time.Duration `env:"LOW_BALANCE_CHECK_INTERVAL" env-default:"10m"`
	NoBudgetCheckInterval          time.Duration `env:"NO_BUDGET_CHECK_INTERVAL" env-default:"10m"`
	CampaignCompletedCheckInterval time.Duration `env:"CAMPAIGN_COMPLETED_CHECK_INTERVAL" env-default:"10m"`
}
