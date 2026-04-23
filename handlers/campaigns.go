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
	"github.com/taslavich/frontBack/models"
	"github.com/taslavich/frontBack/utils"
)

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
