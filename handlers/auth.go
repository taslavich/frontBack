package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/taslavich/frontBack/config"
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"

	"github.com/taslavich/frontBack/middleware"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db  *sql.DB
	cfg *config.ApiConfig
}

func NewAuthHandler(db *sql.DB, cfg *config.ApiConfig) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// POST /api/auth/signup
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email           string `json:"email"`
		Password        string `json:"password"`
		FullName        string `json:"full_name"`
		ManagerTelegram string `json:"manager_telegram"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.Email == "" || body.Password == "" {
		utils.WriteError(w, http.StatusBadRequest, "Email and password required")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := models.User{
		Login:                        body.Email,
		Mail:                         body.Email,
		Name:                         body.FullName,
		ManagerTelegram:              body.ManagerTelegram,
		Timezone:                     "utc_3",
		BalanceTreshold:              100,
		PasswordHash:                 string(hashed),
		EmailNotifications:           true,
		CampaignStatusNotifications:  true,
		LowBalanceNotifications:      true,
		CampaignBalanceNotifications: true,
	}

	var userID string
	err = h.db.QueryRow(`
        INSERT INTO users (login, mail, name, manager_telegram, timezone, balance_treshold, password_hash,
                           email_notifications, campaign_status_notifications, low_balance_notifications,
                           campaign_balance_notifications)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id
    `, user.Login, user.Mail, user.Name, user.ManagerTelegram, user.Timezone, user.BalanceTreshold,
		user.PasswordHash, user.EmailNotifications, user.CampaignStatusNotifications,
		user.LowBalanceNotifications, user.CampaignBalanceNotifications).Scan(&userID)
	if err != nil {
		utils.WriteError(w, http.StatusConflict, "User already exists")
		return
	}
	user.ID = userID

	accessToken, _ := utils.GenerateAccessToken(userID, h.cfg.JWTSecret, h.cfg.AccessTokenTTL)
	refreshToken, _ := utils.GenerateRefreshToken(userID, h.cfg.JWTSecret, h.cfg.RefreshTokenTTL)

	// сохраняем refresh token
	_, _ = h.db.Exec(`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, refreshToken, time.Now().Add(time.Duration(h.cfg.RefreshTokenTTL)*time.Hour))

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

// POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	var user models.User
	err := h.db.QueryRow(`
        SELECT id, login, mail, name, telegram, manager_telegram, balance, timezone,
               email_notifications, campaign_status_notifications, low_balance_notifications,
               campaign_balance_notifications, balance_treshold, password_hash
        FROM users WHERE login = $1
    `, body.Email).Scan(
		&user.ID, &user.Login, &user.Mail, &user.Name, &user.Telegram,
		&user.ManagerTelegram, &user.Balance, &user.Timezone,
		&user.EmailNotifications, &user.CampaignStatusNotifications, &user.LowBalanceNotifications,
		&user.CampaignBalanceNotifications, &user.BalanceTreshold, &user.PasswordHash,
	)
	if err == sql.ErrNoRows {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	accessToken, _ := utils.GenerateAccessToken(user.ID, h.cfg.JWTSecret, h.cfg.AccessTokenTTL)
	refreshToken, _ := utils.GenerateRefreshToken(user.ID, h.cfg.JWTSecret, h.cfg.RefreshTokenTTL)

	// обновляем refresh token
	_, _ = h.db.Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, user.ID)
	_, _ = h.db.Exec(`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		user.ID, refreshToken, time.Now().Add(time.Duration(h.cfg.RefreshTokenTTL)*time.Hour))

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	var userID string
	var expiresAt time.Time
	err := h.db.QueryRow(`SELECT user_id, expires_at FROM refresh_tokens WHERE token = $1`, body.RefreshToken).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows || time.Now().After(expiresAt) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	accessToken, _ := utils.GenerateAccessToken(userID, h.cfg.JWTSecret, h.cfg.AccessTokenTTL)
	newRefreshToken, _ := utils.GenerateRefreshToken(userID, h.cfg.JWTSecret, h.cfg.RefreshTokenTTL)

	// заменяем refresh token
	_, _ = h.db.Exec(`DELETE FROM refresh_tokens WHERE token = $1`, body.RefreshToken)
	_, _ = h.db.Exec(`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, newRefreshToken, time.Now().Add(time.Duration(h.cfg.RefreshTokenTTL)*time.Hour))

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// из контекста достаём userID (middleware)
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	_, _ = h.db.Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/auth/session
func (h *AuthHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var user models.User
	err := h.db.QueryRow(`
        SELECT id, login, mail, name
        FROM users WHERE id = $1
    `, userID).Scan(&user.ID, &user.Login, &user.Mail, &user.Name)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":   user.ID,
		"email":     user.Login,
		"full_name": user.Name,
	})
}

// POST /api/auth/password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if body.NewPassword == "" {
		utils.WriteError(w, http.StatusBadRequest, "New password required")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}
	_, err = h.db.Exec(`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, hashed, userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Could not update password")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
