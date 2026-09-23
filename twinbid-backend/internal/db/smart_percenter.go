package db

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureSmartPercenterSchema installs the PostgreSQL objects owned by the
// cabinet backend for Smart/Percenter campaigns. ORTB/ADV only consumes this
// state and must not own migrations for the users/campaigns tables.
func EnsureSmartPercenterSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_spend_remaining DECIMAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_revision BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS type_model INT NOT NULL DEFAULT 1;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace n ON n.oid = t.relnamespace
				WHERE c.conname = 'campaigns_type_model_check'
				  AND t.relname = 'campaigns'
				  AND n.nspname = current_schema()
			) THEN
				ALTER TABLE campaigns
					ADD CONSTRAINT campaigns_type_model_check
					CHECK (type_model IN (1, 2, 3));
			END IF;
		END $$;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace n ON n.oid = t.relnamespace
				WHERE c.conname = 'users_promo_revision_nonnegative'
				  AND t.relname = 'users'
				  AND n.nspname = current_schema()
			) THEN
				ALTER TABLE users
					ADD CONSTRAINT users_promo_revision_nonnegative
					CHECK (promo_revision >= 0);
			END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS adv_promo_spend_events (
			event_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			campaign_id TEXT NOT NULL,
			spend_delta NUMERIC NOT NULL CHECK (spend_delta > 0),
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_adv_promo_spend_events_applied_at
			ON adv_promo_spend_events(applied_at);`,
		`CREATE OR REPLACE FUNCTION bump_users_promo_revision()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF NEW.promo_revision < OLD.promo_revision THEN
				NEW.promo_revision := OLD.promo_revision;
			END IF;

			IF NEW.promo_spend_remaining IS DISTINCT FROM OLD.promo_spend_remaining
			   AND NEW.promo_revision <= OLD.promo_revision THEN
				NEW.promo_revision := OLD.promo_revision + 1;
			END IF;

			RETURN NEW;
		END;
		$$;`,
		`DROP TRIGGER IF EXISTS users_promo_revision_bump ON users;`,
		`CREATE TRIGGER users_promo_revision_bump
		BEFORE UPDATE OF promo_spend_remaining, promo_revision ON users
		FOR EACH ROW
		EXECUTE FUNCTION bump_users_promo_revision();`,
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("smart percenter schema: %w", err)
		}
	}
	return nil
}
