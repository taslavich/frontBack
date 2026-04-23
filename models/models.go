package models

import (
	"time"
)

type User struct {
	ID                           string    `json:"id" db:"id"`
	Login                        string    `json:"login" db:"login"`
	Mail                         string    `json:"mail" db:"mail"`
	Name                         string    `json:"name" db:"name"`
	Telegram                     *string   `json:"telegram" db:"telegram"`
	ManagerTelegram              string    `json:"manager_telegram" db:"manager_telegram"`
	Balance                      float64   `json:"balance" db:"balance"`
	Timezone                     string    `json:"timezone" db:"timezone"`
	EmailNotifications           bool      `json:"email_notifications" db:"email_notifications"`
	CampaignStatusNotifications  bool      `json:"campaign_status_notifications" db:"campaign_status_notifications"`
	LowBalanceNotifications      bool      `json:"low_balance_notifications" db:"low_balance_notifications"`
	CampaignBalanceNotifications bool      `json:"campaign_balanse_notifications" db:"campaign_balance_notifications"`
	BalanceTreshold              float64   `json:"balance_treshold" db:"balance_treshold"`
	PasswordHash                 string    `json:"-" db:"password_hash"`
	CreatedAt                    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at" db:"updated_at"`
}

type RefreshToken struct {
	UserID    string    `db:"user_id"`
	Token     string    `db:"token"`
	ExpiresAt time.Time `db:"expires_at"`
}
