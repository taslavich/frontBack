package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	c "twinbid-backend/internal/pubcontract"
	"twinbid-backend/internal/publisherconfig"
	"unicode/utf8"
)

type Service struct {
	DB             *sql.DB
	Redis          redis.UniversalClient
	Reports        Reporter
	Config         publisherconfig.Config
	Bot            *bot.BotClient
	AdminID        string
	lastGuardCheck time.Time
}
type Code struct {
	c.Code
	CreatedAt     time.Time `json:"created_at"`
	SyncedVersion int64     `json:"synced_version"`
}

const codeColumns = `ad_code_id::text,user_id::text,short_code,name,format,traffic_type,device,is_active,archived_at IS NOT NULL,trigger_click,frequency_minutes,max_per_session,version,created_at,synced_version`

func scanCode(row interface{ Scan(...any) error }) (Code, error) {
	var a Code
	e := row.Scan(&a.ID, &a.UserID, &a.ShortCode, &a.Name, &a.Format, &a.TrafficType, &a.Device, &a.Active, &a.Archived, &a.TriggerClick, &a.FrequencyMinutes, &a.MaxPerSession, &a.Version, &a.CreatedAt, &a.SyncedVersion)
	return a, e
}
func New(db *sql.DB, r redis.UniversalClient, reports Reporter, cfg publisherconfig.Config, b config.BotConfig) *Service {
	return &Service{DB: db, Redis: r, Reports: reports, Config: cfg, Bot: bot.NewBotClient(b.BaseURL, b.InternalSecret), AdminID: b.AdminUserID}
}
func (s *Service) Routes(r chi.Router) {
	r.Get("/api/publisher/ad-codes", s.listCodes)
	r.Post("/api/publisher/ad-codes", s.createCode)
	r.Get("/api/publisher/ad-codes/{id}", s.getCode)
	r.Get("/api/publisher/ad-codes/{id}/diagnostics", s.diagnostics)
	r.Patch("/api/publisher/ad-codes/{id}", s.patchCode)
	r.Delete("/api/publisher/ad-codes/{id}", s.archiveCode)
	r.Post("/api/publisher/ad-codes/{id}/duplicate", s.duplicateCode)
	r.Get("/api/publisher/sites", s.inventory)
	r.Get("/api/publisher/zones", s.inventory)
	r.Get("/api/publisher/overview", s.overview)
	r.Post("/api/publisher/stats/query", s.queryStats)
	r.Get("/api/publisher/balance", s.balance)
	r.Get("/api/publisher/payouts", s.listPayouts)
	r.Post("/api/publisher/payouts", s.createPayout)
	r.Post("/api/publisher/payouts/{id}/cancel", s.cancelPayout)
	r.Post("/api/publisher/payouts/{id}/decision", s.payoutDecision)
	r.Get("/api/publisher/settings", s.settings)
	r.Patch("/api/publisher/settings", s.settings)
	r.Get("/api/publisher/sync-health", s.health)
	r.Post("/api/publisher/admin/redis-resume", s.resume)
	r.Put("/api/publisher/admin/domain-block", s.domainBlock)
}
func reply(w http.ResponseWriter, v any, e error) {
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			e = httpx.NotFound("not found")
		}
		log.Printf("publisher API: %v", e)
		var he httpx.HTTPError
		if !errors.As(e, &he) {
			e = httpx.ServiceUnavailable("publisher dependency unavailable")
		}
		httpx.Error(w, e)
		return
	}
	httpx.JSON(w, 200, v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return httpx.BadRequest("invalid JSON: " + e.Error())
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return httpx.BadRequest("only one JSON value allowed")
	}
	return nil
}
func (s *Service) ownerCode(r *http.Request) (Code, error) {
	id := chi.URLParam(r, "id")
	if _, e := uuid.Parse(id); e != nil {
		return Code{}, httpx.NotFound("not found")
	}
	v, e := scanCode(s.DB.QueryRowContext(r.Context(), "SELECT "+codeColumns+" FROM ad_codes WHERE ad_code_id=$1 AND user_id=$2", id, auth.UserID(r)))
	if e == nil && v.TrafficType != "adult" && v.TrafficType != "mainstream" {
		return Code{}, httpx.BadRequest("stored traffic_type requires explicit operator resolution")
	}
	return v, e
}
func (s *Service) listCodes(w http.ResponseWriter, r *http.Request) {
	limit, offset, e := pagination(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	rows, e := s.DB.QueryContext(r.Context(), "SELECT "+codeColumns+" FROM ad_codes WHERE user_id=$1 AND traffic_type IN ('adult','mainstream') ORDER BY created_at DESC,ad_code_id LIMIT $2 OFFSET $3", auth.UserID(r), limit, offset)
	if e != nil {
		reply(w, nil, e)
		return
	}
	defer rows.Close()
	items := []Code{}
	for rows.Next() {
		a, e := scanCode(rows)
		if e != nil {
			reply(w, nil, e)
			return
		}
		items = append(items, a)
	}
	reply(w, map[string]any{"items": items, "limit": limit, "offset": offset, "public_script_url": strings.TrimRight(s.Config.PublicURL, "/") + "/static/adManager.js"}, rows.Err())
}
func (s *Service) insertCode(ctx context.Context, user string, a c.Code) (Code, error) {
	if e := a.Validate(); e != nil {
		return Code{}, httpx.BadRequest(e.Error())
	}
	a.ID = uuid.NewString()
	a.ShortCode = "tb_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	return scanCode(s.DB.QueryRowContext(ctx, `INSERT INTO ad_codes(ad_code_id,user_id,short_code,name,format,traffic_type,device,is_active,trigger_click,frequency_minutes,max_per_session) VALUES($1,$2,$3,$4,$5,$6,$7,true,$8,$9,$10) RETURNING `+codeColumns, a.ID, user, a.ShortCode, a.Name, a.Format, a.TrafficType, a.Device, a.TriggerClick, a.FrequencyMinutes, a.MaxPerSession))
}
func (s *Service) createCode(w http.ResponseWriter, r *http.Request) {
	a := c.Code{Format: "popunder", Device: "all", TriggerClick: 1, FrequencyMinutes: 10}
	if e := decode(w, r, &a); e != nil {
		reply(w, nil, e)
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.ID != "" || a.UserID != "" || a.ShortCode != "" || a.Version != 0 || a.Epoch != "" || a.Archived {
		reply(w, nil, httpx.BadRequest("identity and version are server-owned"))
		return
	}
	if e := a.Validate(); e != nil {
		reply(w, nil, httpx.BadRequest(e.Error()))
		return
	}
	v, e := s.insertCode(r.Context(), auth.UserID(r), a)
	reply(w, v, e)
}
func (s *Service) getCode(w http.ResponseWriter, r *http.Request) {
	v, e := s.ownerCode(r)
	reply(w, v, e)
}
func (s *Service) duplicateCode(w http.ResponseWriter, r *http.Request) {
	a, e := s.ownerCode(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	a.Name = strings.TrimSpace(a.Name + " copy")
	if utf8.RuneCountInString(a.Name) > 100 {
		a.Name = string([]rune(a.Name)[:95]) + " copy"
	}
	v, e := s.insertCode(r.Context(), auth.UserID(r), a.Code)
	reply(w, v, e)
}
func (s *Service) patchCode(w http.ResponseWriter, r *http.Request) {
	a, e := s.ownerCode(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	if a.Archived {
		reply(w, nil, httpx.Conflict("archived code"))
		return
	}
	// Version check protects edits from silent overwrite by another browser/tab.
	var fields map[string]json.RawMessage
	if e = decode(w, r, &fields); e != nil {
		reply(w, nil, e)
		return
	}
	if raw, ok := fields["traffic_type"]; ok {
		var traffic string
		if string(raw) == "null" || json.Unmarshal(raw, &traffic) != nil || (traffic != "adult" && traffic != "mainstream") {
			reply(w, nil, httpx.BadRequest("traffic_type must be adult or mainstream"))
			return
		}
	}
	allowed := map[string]bool{"name": true, "traffic_type": true, "device": true, "is_active": true, "trigger_click": true, "frequency_minutes": true, "max_per_session": true, "version": true}
	for k := range fields {
		if !allowed[k] {
			reply(w, nil, httpx.BadRequest("unknown or immutable field: "+k))
			return
		}
	}
	expected := a.Version
	if b, ok := fields["version"]; ok {
		if e = json.Unmarshal(b, &expected); e != nil {
			reply(w, nil, httpx.BadRequest("invalid version"))
			return
		}
		delete(fields, "version")
	}
	b, _ := json.Marshal(fields)
	if e = json.Unmarshal(b, &a.Code); e != nil {
		reply(w, nil, httpx.BadRequest("invalid fields"))
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	if e = a.Validate(); e != nil {
		reply(w, nil, httpx.BadRequest(e.Error()))
		return
	}
	v, e := scanCode(s.DB.QueryRowContext(r.Context(), `UPDATE ad_codes SET name=$3,traffic_type=$4,device=$5,is_active=$6,trigger_click=$7,frequency_minutes=$8,max_per_session=$9,version=version+1,updated_at=now() WHERE ad_code_id=$1 AND user_id=$2 AND version=$10 AND archived_at IS NULL RETURNING `+codeColumns, a.ID, auth.UserID(r), a.Name, a.TrafficType, a.Device, a.Active, a.TriggerClick, a.FrequencyMinutes, a.MaxPerSession, expected))
	if errors.Is(e, sql.ErrNoRows) {
		e = httpx.Conflict("version changed; reload code")
	}
	reply(w, v, e)
}
func (s *Service) archiveCode(w http.ResponseWriter, r *http.Request) {
	a, e := s.ownerCode(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	_, e = s.DB.ExecContext(r.Context(), `UPDATE ad_codes SET archived_at=COALESCE(archived_at,now()),is_active=false,version=version+CASE WHEN archived_at IS NULL THEN 1 ELSE 0 END,updated_at=now() WHERE ad_code_id=$1 AND user_id=$2`, a.ID, auth.UserID(r))
	reply(w, map[string]bool{"archived": true}, e)
}
func (s *Service) isAdmin(r *http.Request) bool {
	return auth.UserID(r) == s.AdminID && s.AdminID != ""
}
func (s *Service) domainBlock(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		reply(w, nil, httpx.Forbidden("admin required"))
		return
	}
	var b struct {
		Hostname string `json:"hostname"`
		Blocked  bool   `json:"blocked"`
		Reason   string `json:"reason"`
	}
	if e := decode(w, r, &b); e != nil {
		reply(w, nil, e)
		return
	}
	host, _, e := c.Page("https://" + b.Hostname + "/")
	if e != nil {
		reply(w, nil, httpx.BadRequest("invalid hostname"))
		return
	}
	_, e = s.DB.ExecContext(r.Context(), `INSERT INTO publisher_domain_blocks(hostname,blocked,reason) VALUES($1,$2,$3) ON CONFLICT(hostname) DO UPDATE SET blocked=EXCLUDED.blocked,reason=EXCLUDED.reason,version=publisher_domain_blocks.version+1,updated_at=now()`, host, b.Blocked, b.Reason)
	reply(w, b, e)
}
func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	var backlog int
	var state, epoch string
	owner := auth.UserID(r)
	if s.isAdmin(r) {
		owner = ""
	}
	e := s.DB.QueryRowContext(r.Context(), `SELECT (SELECT count(*) FROM ad_codes WHERE (version>synced_version OR synced_epoch<>c.epoch) AND ($1='' OR user_id::text=$1))+CASE WHEN $1='' THEN (SELECT count(*) FROM publisher_domain_blocks WHERE version>synced_version OR synced_epoch<>c.epoch) ELSE 0 END,state,epoch FROM publisher_control c WHERE id=1`, owner).Scan(&backlog, &state, &epoch)
	ready, err := s.Redis.Get(r.Context(), c.ReadyKey).Result()
	reply(w, map[string]any{"backlog": backlog, "state": state, "ready": err == nil && ready == epoch, "epoch": func() string {
		if s.isAdmin(r) {
			return epoch
		}
		return ""
	}(), "financial_source_configured": s.RevenueStatus(r.Context()) == "agg_stats_ready", "revenue_status": s.RevenueStatus(r.Context())}, e)
}
func (s *Service) resume(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		reply(w, nil, httpx.Forbidden("admin required"))
		return
	}
	var req struct {
		Epoch         string `json:"epoch"`
		StateRestored bool   `json:"dynamic_state_restored"`
	}
	if e := decode(w, r, &req); e != nil {
		reply(w, nil, e)
		return
	}
	if !req.StateRestored || req.Epoch == "" {
		reply(w, nil, httpx.BadRequest("must confirm restored reservations/counters and current epoch"))
		return
	}
	result, e := s.DB.ExecContext(r.Context(), `UPDATE publisher_control SET state='ready' WHERE id=1 AND epoch=$1 AND state='blocked'`, req.Epoch)
	if e == nil {
		n, _ := result.RowsAffected()
		if n != 1 {
			e = httpx.Conflict("epoch changed or already ready")
		}
	}
	reply(w, map[string]bool{"accepted": e == nil}, e)
}
func (s *Service) settings(w http.ResponseWriter, r *http.Request) {
	user := auth.UserID(r)
	_, e := s.DB.ExecContext(r.Context(), `INSERT INTO publisher_accounts(user_id) VALUES($1) ON CONFLICT DO NOTHING`, user)
	if e != nil {
		reply(w, nil, e)
		return
	}
	if r.Method == "PATCH" {
		var p struct {
			CodeAlerts   bool   `json:"codeAlerts"`
			PayoutMethod string `json:"payoutMethod"`
			Wallet       string `json:"wallet"`
		}
		if e := decode(w, r, &p); e != nil {
			reply(w, nil, e)
			return
		}
		if p.PayoutMethod != "usdt_trc20" && p.PayoutMethod != "usdt_erc20" {
			reply(w, nil, httpx.BadRequest("invalid method"))
			return
		}
		if p.Wallet != "" && !validWallet(p.PayoutMethod, p.Wallet) {
			reply(w, nil, httpx.BadRequest("invalid wallet"))
			return
		}
		b, _ := json.Marshal(p)
		_, e = s.DB.ExecContext(r.Context(), `UPDATE publisher_accounts SET preferences=$2 WHERE user_id=$1`, user, string(b))
		if e != nil {
			reply(w, nil, e)
			return
		}
	}
	var raw []byte
	e = s.DB.QueryRowContext(r.Context(), `SELECT preferences FROM publisher_accounts WHERE user_id=$1`, user).Scan(&raw)
	reply(w, json.RawMessage(raw), e)
}
