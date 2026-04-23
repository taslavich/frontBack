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
		h.enrichCreativeWithURL(r, &cr)
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

	if h.creativeStorage != nil {
		key := generateCreativeObjectKey(cid, c.FileFormat)
		c.S3FilePath = &key
	}

	c.CampaignID = cid
	created, err := scanCreativeRow(h.db.QueryRow(`INSERT INTO creatives (campaign_id,creative_name,link,trackers_macros,w,h,s3_file_path,file_format,title,description) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,campaign_id,creative_name,link,trackers_macros,w,h,s3_file_path,file_format,title,description`, c.CampaignID, c.CreativeName, c.Link, c.TrackersMacros, c.W, c.H, c.S3FilePath, c.FileFormat, c.Title, c.Description))
	if err != nil {
		utils.WriteError(w, 500, "Create failed")
		return
	}
	h.enrichCreativeWithURL(r, &created)
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
	h.enrichCreativeWithURL(r, &updated)
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
	if h.creativeStorage == nil {
		utils.WriteError(w, 500, "S3 integration is not configured")
		return
	}

	var req struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, 400, "Invalid JSON")
		return
	}

	trimmedName := strings.TrimSpace(req.FileName)
	if trimmedName == "" {
		utils.WriteError(w, 400, "file_name is required")
		return
	}
	safeName := strings.ReplaceAll(trimmedName, " ", "_")
	key := fmt.Sprintf("creatives/%d_%s", time.Now().UnixNano(), safeName)
	uploadURL, err := h.creativeStorage.PresignPutObject(r.Context(), key, req.ContentType)
	if err != nil {
		utils.WriteError(w, 500, "Failed to generate S3 upload URL")
		return
	}

	utils.WriteJSON(w, 200, map[string]interface{}{"upload_url": uploadURL, "s3_file_path": key})
}

func (h *MarketingHandler) enrichCreativeWithURL(r *http.Request, cr *models.Creative) {
	if h.creativeStorage == nil || cr.S3FilePath == nil || strings.TrimSpace(*cr.S3FilePath) == "" {
		return
	}
	url, err := h.creativeStorage.PresignGetObject(r.Context(), *cr.S3FilePath)
	if err == nil {
		cr.CreativeURL = &url
	}
}

func generateCreativeObjectKey(campaignID string, fileFormat *string) string {
	ts := time.Now().UnixNano()
	ext := ""
	if fileFormat != nil && strings.TrimSpace(*fileFormat) != "" {
		clean := strings.TrimPrefix(strings.TrimSpace(*fileFormat), ".")
		ext = "." + strings.ToLower(clean)
	}
	return fmt.Sprintf("creatives/%s/%d%s", campaignID, ts, ext)
}
