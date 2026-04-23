package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"
)

func (h *MarketingHandler) ListTopups(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	rows, err := h.db.Query(`SELECT id,user_id,transaction_time,transaction_id,payment_method,bonus_amount,promocode_id,transaction_hash,deposit_amount,total_balance_increase,status,currency,created_at,updated_at FROM user_transactions WHERE user_id=$1 ORDER BY created_at DESC`, uid)
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	defer rows.Close()
	items := []models.UserTransaction{}
	for rows.Next() {
		t, err := scanTopup(rows)
		if err != nil {
			utils.WriteError(w, 500, "Scan error")
			return
		}
		items = append(items, t)
	}
	utils.WriteJSON(w, 200, map[string]interface{}{"items": items, "total": len(items)})
}

func (h *MarketingHandler) CreateTopup(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	var t models.UserTransaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}
	if t.Status == "" {
		t.Status = "draft"
	}
	if t.TransactionTime.IsZero() {
		t.TransactionTime = time.Now()
	}
	created, err := scanTopupRow(h.db.QueryRow(`INSERT INTO user_transactions (user_id,transaction_time,transaction_id,payment_method,bonus_amount,promocode_id,transaction_hash,deposit_amount,total_balance_increase,status,currency) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id,user_id,transaction_time,transaction_id,payment_method,bonus_amount,promocode_id,transaction_hash,deposit_amount,total_balance_increase,status,currency,created_at,updated_at`, uid, t.TransactionTime, t.TransactionID, t.PaymentMethod, t.BonusAmount, t.PromocodeID, t.TransactionHash, t.DepositAmount, t.TotalBalanceIncrease, t.Status, t.Currency))
	if err != nil {
		utils.WriteError(w, 500, "Create failed")
		return
	}
	utils.WriteJSON(w, 200, created)
}

func (h *MarketingHandler) PatchTopup(w http.ResponseWriter, r *http.Request) {
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
	allowed := map[string]string{"status": "status", "transaction_hash": "transaction_hash", "payment_method": "payment_method"}
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
	q := fmt.Sprintf(`UPDATE user_transactions SET %s, updated_at=NOW() WHERE id=$%d AND user_id=$%d RETURNING id,user_id,transaction_time,transaction_id,payment_method,bonus_amount,promocode_id,transaction_hash,deposit_amount,total_balance_increase,status,currency,created_at,updated_at`, strings.Join(sets, ","), i, i+1)
	updated, err := scanTopupRow(h.db.QueryRow(q, args...))
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Topup not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Update failed")
		return
	}
	utils.WriteJSON(w, 200, updated)
}

func (h *MarketingHandler) CancelTopup(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	updated, err := scanTopupRow(h.db.QueryRow(`UPDATE user_transactions SET status='cancelled', updated_at=NOW() WHERE id=$1 AND user_id=$2 RETURNING id,user_id,transaction_time,transaction_id,payment_method,bonus_amount,promocode_id,transaction_hash,deposit_amount,total_balance_increase,status,currency,created_at,updated_at`, id, uid))
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Topup not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Cancel failed")
		return
	}
	utils.WriteJSON(w, 200, updated)
}
