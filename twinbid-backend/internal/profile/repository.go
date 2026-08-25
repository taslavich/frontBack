package profile

import (
	"context"
	"database/sql"
	"errors"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

type queryRunner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *Repository) Get(ctx context.Context, userID string) (models.User, error) {
	return r.get(ctx, r.db, userID, false)
}

func (r *Repository) GetTx(ctx context.Context, tx *sql.Tx, userID string) (models.User, error) {
	return r.get(ctx, tx, userID, true)
}

func (r *Repository) get(ctx context.Context, q queryRunner, userID string, forUpdate bool) (models.User, error) {
	query := selectUser + ` WHERE id=$1`

	if forUpdate {
		query += ` FOR UPDATE`
	}

	row := q.QueryRowContext(ctx, query, userID)

	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.NotFound("user not found")
	}

	return u, err
}

func (r *Repository) ClearAntiPerekrutBlockedTx(ctx context.Context, tx *sql.Tx, userID string) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET antiperekrut_blocked = FALSE, updated_at = NOW()
		WHERE id = $1 AND antiperekrut_blocked = TRUE
	`, userID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) IncreaseGoalTotalTx(ctx context.Context, tx *sql.Tx, userID string, amount float64) (models.User, error) {
	row := tx.QueryRowContext(ctx, `
		UPDATE users
		SET goal_total_dollars = goal_total_dollars + $2,
			low_balance_notified = CASE
				WHEN (goal_total_dollars + $2 - cum_done_dollars) >= balance_treshold THEN false
				ELSE low_balance_notified
			END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, login, mail, name, telegram, manager_telegram,
			goal_total_dollars, cum_done_dollars,
			(goal_total_dollars - cum_done_dollars) AS balance,
			timezone, email_notifications, campaign_status_notifications,
			low_balance_notifications, campaign_balance_notifications,
			balance_treshold, low_balance_notified, partner_id, partner
	`, userID, amount)
	updated, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.NotFound("user not found")
	}
	return updated, err
}

func (r *Repository) Update(ctx context.Context, userID string, u models.User) (models.User, error) {
	return r.update(ctx, r.db, userID, u)
}

func (r *Repository) UpdateTx(ctx context.Context, tx *sql.Tx, userID string, u models.User) (models.User, error) {
	return r.update(ctx, tx, userID, u)
}

func (r *Repository) update(ctx context.Context, q queryRunner, userID string, u models.User) (models.User, error) {
	row := q.QueryRowContext(ctx, `
		UPDATE users
		SET
			login=$2,
			mail=$3,
			name=$4,
			telegram=$5,
			manager_telegram=$6,
			goal_total_dollars=$7,
			cum_done_dollars=$8,
			timezone=$9,
			email_notifications=$10,
			campaign_status_notifications=$11,
			low_balance_notifications=$12,
			campaign_balance_notifications=$13,
			balance_treshold=$14,
			low_balance_notified=$15,
			updated_at=NOW()
		WHERE id=$1
		RETURNING id, login, mail, name, telegram, manager_telegram,
			goal_total_dollars, cum_done_dollars,
			(goal_total_dollars - cum_done_dollars) AS balance,
			timezone, email_notifications, campaign_status_notifications,
			low_balance_notifications, campaign_balance_notifications,
			balance_treshold, low_balance_notified, partner_id, partner
	`, userID, u.Login, u.Mail, u.Name, u.Telegram, u.ManagerTelegram,
		u.GoalTotalDollars, u.CumDoneDollars, u.Timezone,
		u.EmailNotifications, u.CampaignStatusNotifications, u.LowBalanceNotifications,
		u.CampaignBalanseNotifications, u.BalanceTreshold, u.LowBalanceNotified)

	out, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.NotFound("user not found")
	}

	return out, err
}

const selectUser = `SELECT id, login, mail, name, telegram, manager_telegram,
	goal_total_dollars, cum_done_dollars,
	(goal_total_dollars - cum_done_dollars) AS balance,
	timezone, email_notifications, campaign_status_notifications,
	low_balance_notifications, campaign_balance_notifications,
	balance_treshold, low_balance_notified, partner_id, partner FROM users`

type scanner interface{ Scan(dest ...any) error }

func scanUser(s scanner) (models.User, error) {
	var u models.User
	var telegram sql.NullString
	err := s.Scan(
		&u.ID, &u.Login, &u.Mail, &u.Name, &telegram, &u.ManagerTelegram,
		&u.GoalTotalDollars, &u.CumDoneDollars, &u.Balance, &u.Timezone,
		&u.EmailNotifications, &u.CampaignStatusNotifications, &u.LowBalanceNotifications,
		&u.CampaignBalanseNotifications, &u.BalanceTreshold, &u.LowBalanceNotified,
		&u.PartnerID, &u.Partner,
	)
	if err != nil {
		return models.User{}, err
	}
	if telegram.Valid {
		u.Telegram = &telegram.String
	}
	return u, nil
}
