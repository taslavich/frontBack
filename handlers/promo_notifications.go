package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"
)

func (h *MarketingHandler) GetPromocode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var p models.Promocode
	err := h.db.QueryRow(`SELECT id,promocode_text,bonus_percent,usage_count,usage_limit,valid_from,valid_to FROM promocodes WHERE promocode_text=$1`, code).Scan(&p.ID, &p.PromocodeTxt, &p.BonusPercent, &p.UsageCount, &p.UsageLimit, &p.ValidFrom, &p.ValidTo)
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Promocode not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	utils.WriteJSON(w, 200, p)
}

func (h *MarketingHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	rows, err := h.db.Query(`SELECT id,user_id,transaction_id,campaign_id,deposit_amount,status,text,type FROM notifications WHERE user_id=$1 AND status=$2 ORDER BY id DESC`, uid, status)
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	defer rows.Close()
	items := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.TransactionID, &n.CampaignID, &n.DepositAmount, &n.Status, &n.Text, &n.Type); err != nil {
			utils.WriteError(w, 500, "Scan error")
			return
		}
		items = append(items, n)
	}
	utils.WriteJSON(w, 200, items)
}

func (h *MarketingHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	var n models.Notification
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}
	if n.Status == "" {
		n.Status = "active"
	}
	err := h.db.QueryRow(`INSERT INTO notifications (user_id,transaction_id,campaign_id,deposit_amount,status,text,type) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,user_id,transaction_id,campaign_id,deposit_amount,status,text,type`, uid, n.TransactionID, n.CampaignID, n.DepositAmount, n.Status, n.Text, n.Type).Scan(&n.ID, &n.UserID, &n.TransactionID, &n.CampaignID, &n.DepositAmount, &n.Status, &n.Text, &n.Type)
	if err != nil {
		utils.WriteError(w, 500, "Create failed")
		return
	}
	utils.WriteJSON(w, 200, n)
}

func (h *MarketingHandler) PatchNotification(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}
	allowed := map[string]string{"status": "status", "text": "text", "type": "type"}
	sets := []string{}
	args := []interface{}{}
	i := 1
	for k, v := range patch {
		if col, ok := allowed[k]; ok {
			sets = append(sets, fmt.Sprintf("%s=$%d", col, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 0 {
		utils.WriteError(w, 400, "No valid fields")
		return
	}
	args = append(args, id, uid)
	q := fmt.Sprintf(`UPDATE notifications SET %s WHERE id=$%d AND user_id=$%d RETURNING id,user_id,transaction_id,campaign_id,deposit_amount,status,text,type`, strings.Join(sets, ","), i, i+1)
	var n models.Notification
	err := h.db.QueryRow(q, args...).Scan(&n.ID, &n.UserID, &n.TransactionID, &n.CampaignID, &n.DepositAmount, &n.Status, &n.Text, &n.Type)
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Notification not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Update failed")
		return
	}
	utils.WriteJSON(w, 200, n)
}
