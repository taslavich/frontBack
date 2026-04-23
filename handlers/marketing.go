package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"github.com/taslavich/frontBack/middleware"
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"
)

type MarketingHandler struct{ db *sql.DB }

func NewMarketingHandler(db *sql.DB) *MarketingHandler { return &MarketingHandler{db: db} }

func userIDFromCtx(r *http.Request) (string, bool) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	return uid, ok
}

// campaigns
func (h *MarketingHandler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	rows, err := h.db.Query(`SELECT campaign_id,user_id,campaign_name,format_type,brand_name,h,w,status,traffic_type,vertical,pricing_model,base_price_cpm,base_price_cpc,evenness_by_slot_mode,goal_total_dollars,cum_done_dollars,start_ts,end_ts,active_intervals,country,language,device_type,os,browser,site_id,ip FROM campaigns WHERE user_id=$1 ORDER BY start_ts DESC`, uid)
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	defer rows.Close()
	items := []models.Campaign{}
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			utils.WriteError(w, 500, "Scan error")
			return
		}
		items = append(items, c)
	}
	utils.WriteJSON(w, 200, map[string]interface{}{"items": items, "total": len(items)})
}

func (h *MarketingHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	row := h.db.QueryRow(`SELECT campaign_id,user_id,campaign_name,format_type,brand_name,h,w,status,traffic_type,vertical,pricing_model,base_price_cpm,base_price_cpc,evenness_by_slot_mode,goal_total_dollars,cum_done_dollars,start_ts,end_ts,active_intervals,country,language,device_type,os,browser,site_id,ip FROM campaigns WHERE campaign_id=$1 AND user_id=$2`, id, uid)
	c, err := scanCampaignRow(row)
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Campaign not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	utils.WriteJSON(w, 200, c)
}

func (h *MarketingHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	var c models.Campaign
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}
	if c.Status == "" {
		c.Status = "draft"
	}
	if c.StartTS.IsZero() {
		c.StartTS = time.Now()
	}
	if c.EndTS.IsZero() {
		c.EndTS = c.StartTS.Add(24 * time.Hour)
	}
	if c.Country == nil {
		c.Country = map[string]int{}
	}
	if c.Language == nil {
		c.Language = map[string]int{}
	}
	if c.DeviceType == nil {
		c.DeviceType = map[string]int{}
	}
	if c.OS == nil {
		c.OS = map[string]int{}
	}
	if c.Browser == nil {
		c.Browser = map[string]int{}
	}
	if c.SiteID == nil {
		c.SiteID = map[string]int{}
	}
	if c.IP == nil {
		c.IP = map[string]int{}
	}
	row := h.db.QueryRow(`INSERT INTO campaigns (user_id,campaign_name,format_type,brand_name,h,w,status,traffic_type,vertical,pricing_model,base_price_cpm,base_price_cpc,evenness_by_slot_mode,goal_total_dollars,cum_done_dollars,start_ts,end_ts,active_intervals,country,language,device_type,os,browser,site_id,ip) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,0,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24) RETURNING campaign_id,user_id,campaign_name,format_type,brand_name,h,w,status,traffic_type,vertical,pricing_model,base_price_cpm,base_price_cpc,evenness_by_slot_mode,goal_total_dollars,cum_done_dollars,start_ts,end_ts,active_intervals,country,language,device_type,os,browser,site_id,ip`, uid, c.CampaignName, c.FormatType, c.BrandName, c.H, c.W, c.Status, c.TrafficType, pq.Array(c.Vertical), c.PricingModel, c.BasePriceCPM, c.BasePriceCPC, c.EvennessBySlotMode, c.GoalTotalDollars, c.StartTS, c.EndTS, pq.Array(flattenIntervals(c.ActiveIntervals)), c.Country, c.Language, c.DeviceType, c.OS, c.Browser, c.SiteID, c.IP)
	created, err := scanCampaignRow(row)
	if err != nil {
		utils.WriteError(w, 500, "Create failed")
		return
	}
	utils.WriteJSON(w, 200, created)
}

func (h *MarketingHandler) PatchCampaign(w http.ResponseWriter, r *http.Request) {
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
	allowed := map[string]string{"campaign_name": "campaign_name", "status": "status", "traffic_type": "traffic_type", "pricing_model": "pricing_model", "goal_total_dollars": "goal_total_dollars", "base_price_cpm": "base_price_cpm", "base_price_cpc": "base_price_cpc"}
	sets := []string{}
	args := []interface{}{}
	i := 1
	for k, v := range patch {
		if col, ok := allowed[k]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 0 {
		utils.WriteError(w, 400, "No valid fields")
		return
	}
	args = append(args, id, uid)
	q := fmt.Sprintf("UPDATE campaigns SET %s WHERE campaign_id = $%d AND user_id = $%d RETURNING campaign_id,user_id,campaign_name,format_type,brand_name,h,w,status,traffic_type,vertical,pricing_model,base_price_cpm,base_price_cpc,evenness_by_slot_mode,goal_total_dollars,cum_done_dollars,start_ts,end_ts,active_intervals,country,language,device_type,os,browser,site_id,ip", strings.Join(sets, ", "), i, i+1)
	updated, err := scanCampaignRow(h.db.QueryRow(q, args...))
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Campaign not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Update failed")
		return
	}
	utils.WriteJSON(w, 200, updated)
}

func (h *MarketingHandler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	res, err := h.db.Exec(`DELETE FROM campaigns WHERE campaign_id=$1 AND user_id=$2`, id, uid)
	if err != nil {
		utils.WriteError(w, 500, "Delete failed")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		utils.WriteError(w, 404, "Campaign not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// creatives
func (h *MarketingHandler) ListCreatives(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	cid := chi.URLParam(r, "cid")
	rows, err := h.db.Query(`SELECT c.id,c.campaign_id,c.creative_name,c.link,c.trackers_macros,c.w,c.h,c.s3_file_path,c.file_format,c.title,c.description FROM creatives c JOIN campaigns cp ON cp.campaign_id=c.campaign_id WHERE c.campaign_id=$1 AND cp.user_id=$2`, cid, uid)
	if err != nil {
		utils.WriteError(w, 500, "Database error")
		return
	}
	defer rows.Close()
	items := []models.Creative{}
	for rows.Next() {
		cr, err := scanCreative(rows)
		if err != nil {
			utils.WriteError(w, 500, "Scan error")
			return
		}
		items = append(items, cr)
	}
	utils.WriteJSON(w, 200, items)
}
func (h *MarketingHandler) CreateCreative(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	cid := chi.URLParam(r, "cid")
	var exists bool
	if err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM campaigns WHERE campaign_id=$1 AND user_id=$2)`, cid, uid).Scan(&exists); err != nil || !exists {
		utils.WriteError(w, 404, "Campaign not found")
		return
	}
	var c models.Creative
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}
	c.CampaignID = cid
	created, err := scanCreativeRow(h.db.QueryRow(`INSERT INTO creatives (campaign_id,creative_name,link,trackers_macros,w,h,s3_file_path,file_format,title,description) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,campaign_id,creative_name,link,trackers_macros,w,h,s3_file_path,file_format,title,description`, c.CampaignID, c.CreativeName, c.Link, c.TrackersMacros, c.W, c.H, c.S3FilePath, c.FileFormat, c.Title, c.Description))
	if err != nil {
		utils.WriteError(w, 500, "Create failed")
		return
	}
	utils.WriteJSON(w, 200, created)
}
func (h *MarketingHandler) PatchCreative(w http.ResponseWriter, r *http.Request) {
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
	allowed := map[string]string{"creative_name": "creative_name", "link": "link", "title": "title", "description": "description", "s3_file_path": "s3_file_path", "file_format": "file_format"}
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
	q := fmt.Sprintf(`UPDATE creatives c SET %s FROM campaigns cp WHERE c.campaign_id=cp.campaign_id AND c.id=$%d AND cp.user_id=$%d RETURNING c.id,c.campaign_id,c.creative_name,c.link,c.trackers_macros,c.w,c.h,c.s3_file_path,c.file_format,c.title,c.description`, strings.Join(sets, ","), i, i+1)
	updated, err := scanCreativeRow(h.db.QueryRow(q, args...))
	if err == sql.ErrNoRows {
		utils.WriteError(w, 404, "Creative not found")
		return
	}
	if err != nil {
		utils.WriteError(w, 500, "Update failed")
		return
	}
	utils.WriteJSON(w, 200, updated)
}
func (h *MarketingHandler) DeleteCreative(w http.ResponseWriter, r *http.Request) {
	uid, ok := userIDFromCtx(r)
	if !ok {
		utils.WriteError(w, 401, "Unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	res, err := h.db.Exec(`DELETE FROM creatives c USING campaigns cp WHERE c.campaign_id=cp.campaign_id AND c.id=$1 AND cp.user_id=$2`, id, uid)
	if err != nil {
		utils.WriteError(w, 500, "Delete failed")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		utils.WriteError(w, 404, "Creative not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *MarketingHandler) GetUploadURL(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, 200, map[string]interface{}{"upload_url": "https://example-upload.local/presigned", "s3_file_path": fmt.Sprintf("creatives/%d", time.Now().UnixNano()), "expires_in": 900})
}

// topups
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

// promo + notifications
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

func flattenIntervals(in [][2]string) []string {
	out := []string{}
	for _, v := range in {
		out = append(out, v[0], v[1])
	}
	return out
}
func parseIntervals(in []string) [][2]string {
	out := [][2]string{}
	for i := 0; i+1 < len(in); i += 2 {
		out = append(out, [2]string{in[i], in[i+1]})
	}
	return out
}

func scanCampaign(s interface {
	Scan(dest ...interface{}) error
}) (models.Campaign, error) { return scanCampaignRow(s) }
func scanCampaignRow(s interface {
	Scan(dest ...interface{}) error
}) (models.Campaign, error) {
	var c models.Campaign
	var vertical []string
	var intervals []string
	err := s.Scan(&c.CampaignID, &c.UserID, &c.CampaignName, &c.FormatType, &c.BrandName, &c.H, &c.W, &c.Status, &c.TrafficType, pq.Array(&vertical), &c.PricingModel, &c.BasePriceCPM, &c.BasePriceCPC, &c.EvennessBySlotMode, &c.GoalTotalDollars, &c.CumDoneDollars, &c.StartTS, &c.EndTS, pq.Array(&intervals), &c.Country, &c.Language, &c.DeviceType, &c.OS, &c.Browser, &c.SiteID, &c.IP)
	c.Vertical = vertical
	c.ActiveIntervals = parseIntervals(intervals)
	return c, err
}
func scanCreative(s interface {
	Scan(dest ...interface{}) error
}) (models.Creative, error) { return scanCreativeRow(s) }
func scanCreativeRow(s interface {
	Scan(dest ...interface{}) error
}) (models.Creative, error) {
	var c models.Creative
	err := s.Scan(&c.ID, &c.CampaignID, &c.CreativeName, &c.Link, &c.TrackersMacros, &c.W, &c.H, &c.S3FilePath, &c.FileFormat, &c.Title, &c.Description)
	return c, err
}
func scanTopup(s interface {
	Scan(dest ...interface{}) error
}) (models.UserTransaction, error) { return scanTopupRow(s) }
func scanTopupRow(s interface {
	Scan(dest ...interface{}) error
}) (models.UserTransaction, error) {
	var t models.UserTransaction
	err := s.Scan(&t.ID, &t.UserID, &t.TransactionTime, &t.TransactionID, &t.PaymentMethod, &t.BonusAmount, &t.PromocodeID, &t.TransactionHash, &t.DepositAmount, &t.TotalBalanceIncrease, &t.Status, &t.Currency, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
