package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateUser(ctx context.Context, email, password, fullName, managerTelegram string) (models.User, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO users (login, mail, name, manager_telegram, password)
		VALUES ($1, $1, $2, $3, $4)
		RETURNING id, login, mail, name, telegram, manager_telegram, balance, timezone,
			email_notifications, campaign_status_notifications, low_balance_notifications,
			campaign_balance_notifications, balance_treshold
	`, email, fullName, managerTelegram, password)
	u, err := scanUser(row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return models.User{}, httpx.Conflict("user already exists")
		}
		return models.User{}, err
	}
	return u, nil
}

func (r *Repository) GetUserByEmailAndPassword(ctx context.Context, email, password string) (models.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, login, mail, name, telegram, manager_telegram, balance, timezone,
			email_notifications, campaign_status_notifications, low_balance_notifications,
			campaign_balance_notifications, balance_treshold
		FROM users
		WHERE mail = $1 AND password = $2
	`, email, password)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.Unauthorized("invalid email or password")
	}
	return u, err
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (models.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, login, mail, name, telegram, manager_telegram, balance, timezone,
			email_notifications, campaign_status_notifications, low_balance_notifications,
			campaign_balance_notifications, balance_treshold
		FROM users
		WHERE id = $1
	`, userID)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.NotFound("user not found")
	}
	return u, err
}

func (r *Repository) SaveRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, token, expiresAt)
	return err
}

func (r *Repository) HasRefreshToken(ctx context.Context, userID, token string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM refresh_tokens WHERE user_id = $1 AND token = $2 AND expires_at > NOW()
		)
	`, userID, token).Scan(&exists)
	return exists, err
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

func (r *Repository) DeleteRefreshTokensByUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}

func (r *Repository) UpdatePassword(ctx context.Context, userID, newPassword string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET password = $2, updated_at = NOW() WHERE id = $1`, userID, newPassword)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("user not found")
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (models.User, error) {
	var u models.User
	var telegram sql.NullString
	err := row.Scan(
		&u.ID, &u.Login, &u.Mail, &u.Name, &telegram, &u.ManagerTelegram, &u.Balance, &u.Timezone,
		&u.EmailNotifications, &u.CampaignStatusNotifications, &u.LowBalanceNotifications,
		&u.CampaignBalanseNotifications, &u.BalanceTreshold,
	)
	if err != nil {
		return models.User{}, err
	}
	if telegram.Valid {
		u.Telegram = &telegram.String
	}
	return u, nil
}
