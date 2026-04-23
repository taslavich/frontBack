package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/taslavich/frontBack/middleware"
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"
)

type ProfileHandler struct {
	db *sql.DB
}

func NewProfileHandler(db *sql.DB) *ProfileHandler {
	return &ProfileHandler{db: db}
}

// GET /api/profile
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var user models.User
	err := h.db.QueryRow(`
        SELECT id, login, mail, name, telegram, manager_telegram, balance, timezone,
               email_notifications, campaign_status_notifications, low_balance_notifications,
               campaign_balance_notifications, balance_treshold
        FROM users WHERE id = $1
    `, userID).Scan(
		&user.ID, &user.Login, &user.Mail, &user.Name, &user.Telegram,
		&user.ManagerTelegram, &user.Balance, &user.Timezone,
		&user.EmailNotifications, &user.CampaignStatusNotifications, &user.LowBalanceNotifications,
		&user.CampaignBalanceNotifications, &user.BalanceTreshold,
	)
	if err == sql.ErrNoRows {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}
	utils.WriteJSON(w, http.StatusOK, user)
}

// PATCH /api/profile
func (h *ProfileHandler) PatchProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Допустимые поля для обновления
	allowedFields := map[string]string{
		"login":                          "login",
		"mail":                           "mail",
		"name":                           "name",
		"telegram":                       "telegram",
		"manager_telegram":               "manager_telegram",
		"timezone":                       "timezone",
		"email_notifications":            "email_notifications",
		"campaign_status_notifications":  "campaign_status_notifications",
		"low_balance_notifications":      "low_balance_notifications",
		"campaign_balanse_notifications": "campaign_balance_notifications",
		"balance_treshold":               "balance_treshold",
	}

	setParts := []string{}
	args := []interface{}{}
	i := 1
	for key, val := range patch {
		col, ok := allowedFields[key]
		if !ok {
			continue
		}
		setParts = append(setParts, col+" = $"+string(rune(i+'0')))
		args = append(args, val)
		i++
	}
	if len(setParts) == 0 {
		utils.WriteError(w, http.StatusBadRequest, "No valid fields to update")
		return
	}
	args = append(args, userID)
	query := "UPDATE users SET " + join(setParts, ", ") + ", updated_at = NOW() WHERE id = $" + string(rune(i+'0')) +
		" RETURNING id, login, mail, name, telegram, manager_telegram, balance, timezone, email_notifications, campaign_status_notifications, low_balance_notifications, campaign_balance_notifications, balance_treshold"

	var updated models.User
	err := h.db.QueryRow(query, args...).Scan(
		&updated.ID, &updated.Login, &updated.Mail, &updated.Name, &updated.Telegram,
		&updated.ManagerTelegram, &updated.Balance, &updated.Timezone,
		&updated.EmailNotifications, &updated.CampaignStatusNotifications, &updated.LowBalanceNotifications,
		&updated.CampaignBalanceNotifications, &updated.BalanceTreshold,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Update failed")
		return
	}
	utils.WriteJSON(w, http.StatusOK, updated)
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	res := parts[0]
	for i := 1; i < len(parts); i++ {
		res += sep + parts[i]
	}
	return res
}
