package topups

import (
	"context"
	"database/sql"
	"errors"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"

	"github.com/google/uuid"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	rows, err := r.db.QueryContext(ctx, selectTx+` WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UserTransaction
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Create(ctx context.Context, t models.UserTransaction) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO user_transactions (user_id, transaction_id, payment_method, bonus_amount, promocode_id, transaction_hash, deposit_amount, total_balance_increase, status, currency)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, user_id, transaction_time, transaction_id, payment_method, bonus_amount, promocode_id, transaction_hash, deposit_amount, total_balance_increase, status, currency, created_at, updated_at
	`, t.UserID, t.TransactionID, t.PaymentMethod, t.BonusAmount, t.PromocodeID, t.TransactionHash, t.DepositAmount, t.TotalBalanceIncrease, t.Status, t.Currency)
	return scanTx(row)
}

func (r *Repository) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE user_transactions SET status='cancelled', updated_at=NOW()
		WHERE id=$2 AND user_id=$1 AND status IN ('draft','pending')
		RETURNING id, user_id, transaction_time, transaction_id, payment_method, bonus_amount, promocode_id, transaction_hash, deposit_amount, total_balance_increase, status, currency, created_at, updated_at
	`, userID, id)
	t, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("topup not found or cannot be cancelled")
	}
	return t, err
}

func (r *Repository) UserUsedPromocode(ctx context.Context, userID, promocodeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_transactions WHERE user_id=$1 AND promocode_id=$2 AND status <> 'cancelled')`, userID, promocodeID).Scan(&exists)
	return exists, err
}

func (r *Repository) Approve(ctx context.Context, userID, topupID string, promoRepo interface {
	IncrementUsage(context.Context, *sql.Tx, string) error
}) (models.UserTransaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
		UPDATE user_transactions SET status='approved', updated_at=NOW()
		WHERE id=$2 AND user_id=$1 AND status <> 'approved'
		RETURNING id, user_id, transaction_time, transaction_id, payment_method, bonus_amount, promocode_id, transaction_hash, deposit_amount, total_balance_increase, status, currency, created_at, updated_at
	`, userID, topupID)
	t, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("topup not found or already approved")
	}
	if err != nil {
		return models.UserTransaction{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET balance = balance + $2,
			updated_at=NOW()
		WHERE id=$1
	`, userID, t.TotalBalanceIncrease); err != nil {
		return models.UserTransaction{}, err
	}
	if t.PromocodeID != nil && *t.PromocodeID != "" {
		if err := promoRepo.IncrementUsage(ctx, tx, *t.PromocodeID); err != nil {
			return models.UserTransaction{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.UserTransaction{}, err
	}
	return t, nil
}

const selectTx = `SELECT id, user_id, transaction_time, transaction_id, payment_method, bonus_amount, promocode_id, transaction_hash, deposit_amount, total_balance_increase, status, currency, created_at, updated_at FROM user_transactions`

type scanner interface{ Scan(dest ...any) error }

func scanTx(s scanner) (models.UserTransaction, error) {
	var t models.UserTransaction
	var promo, hash sql.NullString
	var status string
	err := s.Scan(&t.ID, &t.UserID, &t.TransactionTime, &t.TransactionID, &t.PaymentMethod, &t.BonusAmount, &promo, &hash, &t.DepositAmount, &t.TotalBalanceIncrease, &status, &t.Currency, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if promo.Valid {
		t.PromocodeID = &promo.String
	}
	if hash.Valid {
		t.TransactionHash = &hash.String
	}
	t.Status = models.TopupStatus(status)
	return t, nil
}

func NewTransactionID() string { return uuid.NewString() }


