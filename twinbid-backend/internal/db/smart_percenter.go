package db

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureSmartPercenterSchema installs the additive columns required by the
// Smart CPM percenter. It is intentionally separate from the legacy migration
// list so it can be safely deployed on installations with local migration edits.
func EnsureSmartPercenterSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS promo_spend_remaining DECIMAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS type_model INT NOT NULL DEFAULT 1;`,
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("smart percenter schema: %w", err)
		}
	}
	return nil
}
