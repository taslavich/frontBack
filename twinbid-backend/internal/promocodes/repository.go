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
	row := r.db.QueryRowContext(ctx, selectPromocode+` WHERE UPPER(promocode_text)=UPPER($1)`, strings.TrimSpace(code))
	return scanPromocodeOrNotFound(row)
}

func (r *Repository) GetByCodeForUpdateTx(ctx context.Context, tx *sql.Tx, code string) (models.Promocode, error) {
	row := tx.QueryRowContext(ctx, selectPromocode+` WHERE UPPER(promocode_text)=UPPER($1) FOR UPDATE`, strings.TrimSpace(code))
	return scanPromocodeOrNotFound(row)
}

func (r *Repository) GetByIDForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (models.Promocode, error) {
	row := tx.QueryRowContext(ctx, selectPromocode+` WHERE id=$1 FOR UPDATE`, id)
	return scanPromocodeOrNotFound(row)
}

// ReserveUsageTx atomically consumes one promocode slot. The caller must keep
// this operation in the same transaction as creating/claiming the topup.
func (r *Repository) ReserveUsageTx(ctx context.Context, tx *sql.Tx, promocodeID string) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE promocodes
		SET usage_count = usage_count + 1
		WHERE id=$1 AND (usage_limit IS NULL OR usage_count < usage_limit)
	`, promocodeID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return httpx.BadRequest("promocode usage limit exceeded")
	}
	return nil
}

func (r *Repository) ReleaseUsageTx(ctx context.Context, tx *sql.Tx, promocodeID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE promocodes
		SET usage_count = GREATEST(usage_count - 1, 0)
		WHERE id=$1
	`, promocodeID)
	return err
}

// Kept for compatibility with non-topup code.
func (r *Repository) IncrementUsage(ctx context.Context, promocodeID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE promocodes SET usage_count = usage_count + 1 WHERE id=$1`, promocodeID)
	return err
}

func (r *Repository) IncrementUsageTx(ctx context.Context, tx *sql.Tx, promocodeID string) error {
	return r.ReserveUsageTx(ctx, tx, promocodeID)
}

const selectPromocode = `SELECT id, promocode_text, bonus_percent, usage_count, usage_limit, valid_from, valid_to FROM promocodes`

type scanner interface{ Scan(dest ...any) error }

func scanPromocodeOrNotFound(s scanner) (models.Promocode, error) {
	p, err := scanPromocode(s)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Promocode{}, httpx.NotFound("promocode not found")
	}
	return p, err
}

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
