package promocodes

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) GetByCode(ctx context.Context, code string) (models.Promocode, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, promocode_text, bonus_percent, usage_count, usage_limit, valid_from, valid_to
		FROM promocodes WHERE UPPER(promocode_text)=UPPER($1)
	`, strings.TrimSpace(code))
	p, err := scanPromocode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Promocode{}, httpx.NotFound("promocode not found")
	}
	return p, err
}

func (r *Repository) IncrementUsage(ctx context.Context, promocodeID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE promocodes SET usage_count = usage_count + 1 WHERE id=$1`, promocodeID)
	return err
}

func (r *Repository) IncrementUsageTx(ctx context.Context, tx *sql.Tx, promocodeID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE promocodes SET usage_count = usage_count + 1 WHERE id=$1`, promocodeID)
	return err
}

type scanner interface{ Scan(dest ...any) error }

func scanPromocode(s scanner) (models.Promocode, error) {
	var p models.Promocode
	var limit sql.NullInt64
	var vf, vt sql.NullTime
	err := s.Scan(&p.ID, &p.PromocodeText, &p.BonusPercent, &p.UsageCount, &limit, &vf, &vt)
	if err != nil {
		return models.Promocode{}, err
	}
	if limit.Valid {
		v := int(limit.Int64)
		p.UsageLimit = &v
	}
	if vf.Valid {
		p.ValidFrom = &vf.Time
	}
	if vt.Valid {
		p.ValidTo = &vt.Time
	}
	return p, nil
}
