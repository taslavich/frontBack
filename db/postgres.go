package db

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func NewPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	queries := []string{
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
            password_hash TEXT NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        );`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token TEXT PRIMARY KEY,
            expires_at TIMESTAMP WITH TIME ZONE NOT NULL
        );`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("Migration error: %v", err)
			return err
		}
	}
	return nil
}
