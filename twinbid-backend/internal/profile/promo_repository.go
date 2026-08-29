package profile

import (
	"context"
	"database/sql"
	"errors"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

// IncreaseGoalTotalAndPromoTx credits the normal balance and, independently,
// the amount whose future spend must use promo margin rules. The returned user
// uses the legacy profile projection; promo_spend_remaining is internal state.
func (r *Repository) IncreaseGoalTotalAndPromoTx(ctx context.Context, tx *sql.Tx, userID string, amount, promoAmount float64) (models.User, error) {
	row := tx.QueryRowContext(ctx, `
        UPDATE users
        SET goal_total_dollars = goal_total_dollars + $2,
            promo_spend_remaining = promo_spend_remaining + $3,
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
            balance_treshold, low_balance_notified
    `, userID, amount, promoAmount)
	updated, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, httpx.NotFound("user not found")
	}
	return updated, err
}
