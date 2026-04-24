package db

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func NewPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			login TEXT UNIQUE NOT NULL,
			mail TEXT NOT NULL,
			name TEXT NOT NULL,
			telegram TEXT,
			manager_telegram TEXT NOT NULL,
			balance DECIMAL(12,2) NOT NULL DEFAULT 0,
			timezone TEXT NOT NULL DEFAULT 'utc_3',
			email_notifications BOOLEAN NOT NULL DEFAULT true,
			campaign_status_notifications BOOLEAN NOT NULL DEFAULT true,
			low_balance_notifications BOOLEAN NOT NULL DEFAULT true,
			campaign_balance_notifications BOOLEAN NOT NULL DEFAULT true,
			balance_treshold DECIMAL(10,2) NOT NULL DEFAULT 100,
			password TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT PRIMARY KEY,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS campaigns (
			campaign_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			campaign_name TEXT NOT NULL,
			format_type TEXT NOT NULL,
			brand_name TEXT,
			h INT,
			w INT,
			status TEXT NOT NULL DEFAULT 'draft',
			traffic_type TEXT NOT NULL,
			vertical TEXT[] NOT NULL DEFAULT '{}',
			pricing_model TEXT NOT NULL,
			base_price_cpm DECIMAL(12,4) NOT NULL DEFAULT 0,
			base_price_cpc DECIMAL(12,4) NOT NULL DEFAULT 0,
			evenness_by_slot_mode BOOLEAN NOT NULL DEFAULT false,
			goal_total_dollars DECIMAL(12,2) NOT NULL DEFAULT 0,
			cum_done_dollars DECIMAL(12,2) NOT NULL DEFAULT 0,
			start_ts TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			end_ts TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			active_intervals JSONB NOT NULL DEFAULT '[]'::jsonb,
			country JSONB NOT NULL DEFAULT '{}'::jsonb,
			language JSONB NOT NULL DEFAULT '{}'::jsonb,
			device_type JSONB NOT NULL DEFAULT '{}'::jsonb,
			os JSONB NOT NULL DEFAULT '{}'::jsonb,
			browser JSONB NOT NULL DEFAULT '{}'::jsonb,
			site_id JSONB NOT NULL DEFAULT '{}'::jsonb,
			ip JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS creatives (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			campaign_id UUID NOT NULL REFERENCES campaigns(campaign_id) ON DELETE CASCADE,
			creative_name TEXT NOT NULL,
			link TEXT NOT NULL,
			trackers_macros JSONB NOT NULL DEFAULT '{}'::jsonb,
			w INT,
			h INT,
			name TEXT,
			s3_file_path TEXT,
			file_format TEXT,
			title TEXT,
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS promocodes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			promocode_text TEXT UNIQUE NOT NULL,
			bonus_percent DECIMAL(6,2) NOT NULL DEFAULT 0,
			usage_count INT NOT NULL DEFAULT 0,
			usage_limit INT,
			valid_from TIMESTAMP WITH TIME ZONE,
			valid_to TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS user_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			transaction_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			transaction_id TEXT NOT NULL UNIQUE,
			payment_method TEXT NOT NULL,
			bonus_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
			promocode_id UUID REFERENCES promocodes(id) ON DELETE SET NULL,
			transaction_hash TEXT,
			deposit_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
			total_balance_increase DECIMAL(12,2) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			currency TEXT NOT NULL DEFAULT 'USD',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			transaction_id UUID REFERENCES user_transactions(id) ON DELETE SET NULL,
			campaign_id UUID REFERENCES campaigns(campaign_id) ON DELETE SET NULL,
			deposit_amount DECIMAL(12,2),
			status TEXT NOT NULL DEFAULT 'active',
			text TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_user_id ON campaigns(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_creatives_campaign_id ON creatives(campaign_id);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON user_transactions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_status ON notifications(user_id, status);`,
		`INSERT INTO promocodes (promocode_text, bonus_percent, usage_limit) VALUES
			('TWINBID25', 25, NULL),
			('WELCOME10', 10, NULL)
		ON CONFLICT (promocode_text) DO NOTHING;`,
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			log.Printf("migration error: %v", err)
			return err
		}
	}
	return nil
}
