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
	for _, q := range smartPercenterSchemaQueries() {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("smart percenter schema: %w", err)
		}
	}
	return nil
}

func smartPercenterSchemaQueries() []string {
	return []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_spend_remaining DECIMAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_revision BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_generation BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_spend_synced DECIMAL;`,
		`UPDATE users
		 SET promo_spend_synced = cum_done_dollars
		 WHERE promo_spend_synced IS NULL;`,
		`ALTER TABLE users ALTER COLUMN promo_spend_synced SET DEFAULT 0;`,
		`ALTER TABLE users ALTER COLUMN promo_spend_synced SET NOT NULL;`,
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
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace n ON n.oid = t.relnamespace
				WHERE c.conname = 'users_promo_generation_nonnegative'
				  AND t.relname = 'users'
				  AND n.nspname = current_schema()
			) THEN
				ALTER TABLE users
					ADD CONSTRAINT users_promo_generation_nonnegative
					CHECK (promo_generation >= 0);
			END IF;
		END $$;`,
		`CREATE OR REPLACE FUNCTION bump_users_promo_revision()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF NEW.promo_generation < OLD.promo_generation THEN
				NEW.promo_generation := OLD.promo_generation;
			END IF;

			-- A new promo period starts only when an exhausted/inactive promo
			-- becomes active again. Top-ups while promo is already active stay in
			-- the same generation so in-flight spend from that active period still
			-- consumes the shared promo allowance.
			IF OLD.promo_spend_remaining <= 0
			   AND NEW.promo_spend_remaining > 0
			   AND NEW.promo_generation <= OLD.promo_generation THEN
				NEW.promo_generation := OLD.promo_generation + 1;
			END IF;

			IF NEW.promo_revision < OLD.promo_revision THEN
				NEW.promo_revision := OLD.promo_revision;
			END IF;

			IF (NEW.promo_spend_remaining IS DISTINCT FROM OLD.promo_spend_remaining
			    OR NEW.promo_generation IS DISTINCT FROM OLD.promo_generation)
			   AND NEW.promo_revision <= OLD.promo_revision THEN
				NEW.promo_revision := OLD.promo_revision + 1;
			END IF;

			RETURN NEW;
		END;
		$$;`,
		`CREATE OR REPLACE TRIGGER users_promo_revision_bump
			BEFORE UPDATE OF promo_spend_remaining, promo_revision, promo_generation ON users
			FOR EACH ROW
			EXECUTE FUNCTION bump_users_promo_revision();`,
	}
}
