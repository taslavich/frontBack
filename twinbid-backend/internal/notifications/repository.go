package notifications

import (
	"context"
	"database/sql"
	"errors"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) EnsureAutomatic(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (user_id, status, text, type)
		SELECT u.id, 'active', 'Low balance', 'low_balance'
		FROM users u
		WHERE u.id=$1
		  AND u.low_balance_notifications = true
		  AND (u.goal_total_dollars - u.cum_done_dollars) < u.balance_treshold
		  AND NOT EXISTS (
			SELECT 1 FROM notifications n WHERE n.user_id=u.id AND n.type='low_balance' AND n.status='active'
		  )
	`, userID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO notifications (user_id, campaign_id, status, text, type)
		SELECT c.user_id, c.campaign_id, 'active', 'Low campaign balance', 'other'
		FROM campaigns c
		JOIN users u ON u.id = c.user_id
		WHERE c.user_id=$1
		  AND u.campaign_balance_notifications = true
		  AND c.goal_total_dollars > 0
		  AND c.cum_done_dollars >= c.goal_total_dollars
		  AND c.status IN ('active', 'moderation')
		  AND NOT EXISTS (
			SELECT 1 FROM notifications n WHERE n.user_id=c.user_id AND n.campaign_id=c.campaign_id AND n.text='Low campaign balance' AND n.status='active'
		  )
	`, userID)
	return err
}

func (r *Repository) List(ctx context.Context, userID, status string) ([]models.Notification, error) {
	query := selectNotification + ` WHERE user_id=$1`
	args := []any{userID}
	if status != "" {
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) Create(ctx context.Context, n models.Notification) (models.Notification, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO notifications (user_id, transaction_id, campaign_id, deposit_amount, status, text, type)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, user_id, transaction_id, campaign_id, deposit_amount, status, text, type
	`, n.UserID, n.TransactionID, n.CampaignID, n.DepositAmount, n.Status, n.Text, n.Type)
	return scanNotification(row)
}

func (r *Repository) Patch(ctx context.Context, userID, id string, patch PatchNotificationRequest) (models.Notification, error) {
	current, err := r.Get(ctx, userID, id)
	if err != nil {
		return models.Notification{}, err
	}
	if patch.TransactionIDSet {
		current.TransactionID = patch.TransactionID
	}
	if patch.CampaignIDSet {
		current.CampaignID = patch.CampaignID
	}
	if patch.DepositAmountSet {
		current.DepositAmount = patch.DepositAmount
	}
	if patch.Status != nil {
		current.Status = *patch.Status
	}
	if patch.Text != nil {
		current.Text = *patch.Text
	}
	if patch.Type != nil {
		current.Type = *patch.Type
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE notifications SET transaction_id=$3, campaign_id=$4, deposit_amount=$5, status=$6, text=$7, type=$8, updated_at=NOW()
		WHERE user_id=$1 AND id=$2
		RETURNING id, user_id, transaction_id, campaign_id, deposit_amount, status, text, type
	`, userID, id, current.TransactionID, current.CampaignID, current.DepositAmount, current.Status, current.Text, current.Type)
	return scanNotification(row)
}

func (r *Repository) Get(ctx context.Context, userID, id string) (models.Notification, error) {
	row := r.db.QueryRowContext(ctx, selectNotification+` WHERE user_id=$1 AND id=$2`, userID, id)
	n, err := scanNotification(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Notification{}, httpx.NotFound("notification not found")
	}
	return n, err
}

func (r *Repository) CreateLowBalanceIfNeeded(ctx context.Context, userID string, balance, threshold float64) error {
	if threshold <= 0 || balance >= threshold {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (user_id, status, text, type)
		SELECT $1, 'active', $2, 'low_balance'
		WHERE NOT EXISTS (
			SELECT 1 FROM notifications WHERE user_id=$1 AND type='low_balance' AND status='active'
		)
	`, userID, "Low balance")
	return err
}

func (r *Repository) CreateLowCampaignBalanceIfNeeded(ctx context.Context, userID, campaignID string, leftAmount float64) error {
	if leftAmount > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (user_id, campaign_id, status, text, type)
		SELECT $1, $2, 'active', $3, 'other'
		WHERE NOT EXISTS (
			SELECT 1 FROM notifications WHERE user_id=$1 AND campaign_id=$2 AND type='other' AND status='active'
		)
	`, userID, campaignID, "Low campaign balance")
	return err
}

const selectNotification = `SELECT id, user_id, transaction_id, campaign_id, deposit_amount, status, text, type FROM notifications`

type scanner interface{ Scan(dest ...any) error }

func scanNotification(s scanner) (models.Notification, error) {
	var n models.Notification
	var tx, campaign sql.NullString
	var dep sql.NullFloat64
	err := s.Scan(&n.ID, &n.UserID, &tx, &campaign, &dep, &n.Status, &n.Text, &n.Type)
	if err != nil {
		return models.Notification{}, err
	}
	if tx.Valid {
		n.TransactionID = &tx.String
	}
	if campaign.Valid {
		n.CampaignID = &campaign.String
	}
	if dep.Valid {
		n.DepositAmount = &dep.Float64
	}
	return n, nil
}
