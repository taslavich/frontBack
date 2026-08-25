package partners

import (
	"context"
	"database/sql"
	"errors"

	"twinbid-backend/internal/httpx"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Stats(ctx context.Context, userID string) (Stats, error) {
	var out Stats
	err := r.db.QueryRowContext(ctx, `
		SELECT
			u.partner_id,
			COUNT(referred.id),
			COALESCE(SUM(referred.cum_done_dollars), 0),
			u.partner_withdrawn_dollars
		FROM users AS u
		LEFT JOIN users AS referred
			ON referred.partner = u.partner_id
		WHERE u.id = $1
		GROUP BY u.id, u.partner_id, u.partner_withdrawn_dollars
	`, userID).Scan(
		&out.Partner,
		&out.Advertisers,
		&out.Turnover,
		&out.Withdrawn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Stats{}, httpx.NotFound("user not found")
	}
	return out, err
}
