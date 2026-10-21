package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"time"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type StatsRequest struct {
	LifetimeRevenueOnly bool      `json:"-"`
	BucketIDs           []string  `json:"-"`
	DiscoveredOnly      bool      `json:"-"`
	Countries           []string  `json:"countries"`
	Devices             []string  `json:"devices"`
	OSes                []string  `json:"oses"`
	Browsers            []string  `json:"browsers"`
	From                time.Time `json:"from"`
	To                  time.Time `json:"to"`
	GroupBy             string    `json:"group_by"`
	Timezone            string    `json:"timezone"`
	SiteID              string    `json:"site_id"`
	ZoneID              string    `json:"zone_id"`
	CodeID              string    `json:"ad_code_id"`
	Limit               int       `json:"limit"`
	Offset              int       `json:"offset"`
}
type Metrics struct {
	Fills       uint64  `json:"fills"`
	NoFills     uint64  `json:"no_fills"`
	Requests    uint64  `json:"requests"`
	Errors      uint64  `json:"errors"`
	Impressions uint64  `json:"impressions"`
	Clicks      uint64  `json:"clicks"`
	CTR         float64 `json:"ctr"`
	Revenue     string  `json:"revenue_usd"`
}
type Row struct {
	FirstRequestAt *time.Time `json:"first_request_at,omitempty"`
	MetricsStatus  string     `json:"metrics_status,omitempty"`
	Bucket         string     `json:"bucket"`
	SiteID         string     `json:"site_id"`
	ZoneID         string     `json:"zone_id"`
	CodeID         string     `json:"ad_code_id"`
	Hostname       string     `json:"hostname"`
	LastRequestAt  *time.Time `json:"last_request_at"`
	Metrics
}
type Report struct {
	EffectiveFrom  time.Time `json:"effective_from"`
	EffectiveTo    time.Time `json:"effective_to"`
	TimeResolution string    `json:"time_resolution"`
	Rows           []Row     `json:"rows"`
	Totals         Metrics   `json:"totals"`
	Limit          int       `json:"limit"`
	Offset         int       `json:"offset"`
	RevenueStatus  string    `json:"revenue_status"`
}
type Reporter interface {
	PublisherQuery(context.Context, string, StatsRequest) (Report, error)
}

func pagination(r *http.Request) (int, int, error) {
	limit := 100
	offset := 0
	var e error
	if q := r.URL.Query().Get("limit"); q != "" {
		limit, e = strconv.Atoi(q)
		if e != nil {
			return 0, 0, httpx.BadRequest("invalid limit")
		}
	}
	if q := r.URL.Query().Get("offset"); q != "" {
		offset, e = strconv.Atoi(q)
		if e != nil {
			return 0, 0, httpx.BadRequest("invalid offset")
		}
	}
	if limit < 1 || limit > 500 || offset < 0 || offset > 1000000 {
		return 0, 0, httpx.BadRequest("limit=1..500; offset=0..1000000")
	}
	return limit, offset, nil
}
func (s *Service) report(ctx context.Context, user string, q StatsRequest) (Report, error) {
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Timezone == "" {
		if e := s.DB.QueryRowContext(ctx, `SELECT timezone FROM users WHERE id=$1`, user).Scan(&q.Timezone); e != nil {
			return Report{}, e
		}
		if q.Timezone == "" {
			q.Timezone = "UTC"
		}
	}
	tz, err := NormalizeTimezone(q.Timezone)
	if err != nil {
		return Report{}, httpx.BadRequest("invalid timezone")
	}
	q.Timezone = tz
	v, e := s.Reports.PublisherQuery(ctx, user, q)
	if e != nil {
		return v, e
	}
	if e != nil {
		return v, e
	}
	v.RevenueStatus = s.RevenueStatus(ctx)
	return v, nil
}
func (s *Service) queryStats(w http.ResponseWriter, r *http.Request) {
	var q StatsRequest
	if e := decode(w, r, &q); e != nil {
		reply(w, nil, e)
		return
	}
	v, e := s.report(r.Context(), auth.UserID(r), q)
	reply(w, v, e)
}
func (s *Service) inventory(w http.ResponseWriter, r *http.Request) {
	limit, offset, e := pagination(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	group := "zone"
	if r.URL.Path == "/api/publisher/sites" {
		group = "site"
	}
	provider, ok := s.Reports.(interface {
		PublisherInventory(context.Context, string, string, string, string, int, int) ([]Row, error)
	})
	if !ok {
		reply(w, nil, httpx.ServiceUnavailable("inventory unavailable"))
		return
	}
	items, e := provider.PublisherInventory(r.Context(), auth.UserID(r), group, r.URL.Query().Get("site_id"), r.URL.Query().Get("ad_code_id"), limit, offset)
	if e != nil {
		reply(w, nil, e)
		return
	}
	// Recent activity is scoped to six months; earned aggregate revenue is lifetime.
	bucketIDs := []string{}
	for _, item := range items {
		bucketIDs = append(bucketIDs, item.Bucket)
	}
	if len(items) == 0 {
		reply(w, map[string]any{"rows": items, "limit": limit, "offset": offset, "metric_period": "last_6_months", "revenue_period": "lifetime"}, nil)
		return
	}
	metricReport, err := s.report(r.Context(), auth.UserID(r), StatsRequest{From: time.Now().UTC().AddDate(0, -6, 0), To: time.Now().UTC(), GroupBy: group, Timezone: "UTC", SiteID: r.URL.Query().Get("site_id"), CodeID: r.URL.Query().Get("ad_code_id"), Limit: 500, BucketIDs: bucketIDs})
	if err != nil {
		reply(w, nil, err)
		return
	}
	metrics := map[string]Metrics{}
	for _, v := range metricReport.Rows {
		metrics[v.Bucket] = v.Metrics
	}
	lifetime, e := s.Reports.PublisherQuery(r.Context(), auth.UserID(r), StatsRequest{From: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Now().UTC().Add(time.Hour), GroupBy: group, Timezone: "UTC", Limit: 500, BucketIDs: bucketIDs, LifetimeRevenueOnly: true})
	if e != nil {
		reply(w, nil, e)
		return
	}
	lifetimeRevenue := map[string]string{}
	for _, row := range lifetime.Rows {
		lifetimeRevenue[row.Bucket] = row.Revenue
	}
	for i := range items {
		items[i].Revenue = "0"
		items[i].MetricsStatus = "last_6_months"
		if value, ok := metrics[items[i].Bucket]; ok {
			items[i].Metrics = value
		} else {
			items[i].MetricsStatus = "no_events_in_selected_period"
		}
		items[i].Revenue = lifetimeRevenue[items[i].Bucket]
		if items[i].Revenue == "" {
			items[i].Revenue = "0"
		}
	}
	reply(w, map[string]any{"rows": items, "limit": limit, "offset": offset, "metric_period": "last_6_months", "revenue_period": "lifetime"}, nil)
}
func (s *Service) overview(w http.ResponseWriter, r *http.Request) {
	q := StatsRequest{From: time.Now().UTC().Add(-7 * 24 * time.Hour), To: time.Now().UTC(), GroupBy: "site", Limit: 5}
	v, e := s.report(r.Context(), auth.UserID(r), q)
	if e != nil {
		reply(w, nil, e)
		return
	}
	provider, ok := s.Reports.(interface {
		PublisherTopSites(context.Context, string, time.Time, time.Time) ([]string, error)
	})
	if !ok {
		reply(w, nil, httpx.ServiceUnavailable("publisher aggregate top sites unavailable"))
		return
	}
	top, e := provider.PublisherTopSites(r.Context(), auth.UserID(r), q.From, q.To)
	if e != nil {
		reply(w, nil, e)
		return
	}
	if len(top) > 0 {
		v.Rows = []Row{}
		for _, id := range top {
			single := q
			single.SiteID = id
			single.Limit = 1
			report, err := s.report(r.Context(), auth.UserID(r), single)
			if err != nil {
				reply(w, nil, err)
				return
			}
			v.Rows = append(v.Rows, report.Rows...)
		}
	}
	balance, e := s.Balance(r.Context(), auth.UserID(r))
	reply(w, map[string]any{"report": v, "balance": balance, "from": q.From, "to": q.To}, e)
}
func (s *Service) Notify(ctx context.Context) error {
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked bool
	if e = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(871922134)`).Scan(&locked); e != nil || !locked {
		return e
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn.ExecContext(c, `SELECT pg_advisory_unlock(871922134)`)
	}()
	rows, e := conn.QueryContext(ctx, `SELECT id::text,user_id::text,status,amount::text,method,wallet FROM publisher_payouts WHERE notified_at IS NULL ORDER BY created_at LIMIT 50`)
	if e != nil {
		return e
	}
	type notice struct{ id, user, status, amount, method, wallet string }
	pending := []notice{}
	for rows.Next() {
		var n notice
		if e = rows.Scan(&n.id, &n.user, &n.status, &n.amount, &n.method, &n.wallet); e != nil {
			rows.Close()
			return e
		}
		pending = append(pending, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, n := range pending {
		if e = s.Bot.SendTextMessage(ctx, fmt.Sprintf("Publisher payout %s: %s, $%s %s; user %s; wallet %s. Operator decision: /api/publisher/payouts/%s/decision", n.id, n.status, n.amount, n.method, n.user, n.wallet, n.id)); e != nil {
			return e
		}
		if _, e = conn.ExecContext(ctx, `UPDATE publisher_payouts SET notified_at=now() WHERE id=$1 AND status=$2`, n.id, n.status); e != nil {
			return e
		}
	}
	// CH outages abort monitoring; they must never turn all zones into missing sites.
	users, e := conn.QueryContext(ctx, `SELECT DISTINCT user_id::text FROM ad_codes WHERE is_active AND archived_at IS NULL`)
	if e != nil {
		return e
	}
	ids := []string{}
	for users.Next() {
		var u string
		if e = users.Scan(&u); e != nil {
			users.Close()
			return e
		}
		ids = append(ids, u)
	}
	e = users.Err()
	users.Close()
	if e != nil {
		return e
	}
	for _, user := range ids {
		var preferences []byte
		e = conn.QueryRowContext(ctx, `SELECT preferences FROM publisher_accounts WHERE user_id=$1`, user).Scan(&preferences)
		if e != nil && !errorsNoRows(e) {
			return e
		}
		var prefs struct {
			CodeAlerts *bool `json:"codeAlerts"`
		}
		json.Unmarshal(preferences, &prefs)
		if prefs.CodeAlerts != nil && !*prefs.CodeAlerts {
			continue
		}
		provider, ok := s.Reports.(ZoneReporter)
		if !ok {
			return fmt.Errorf("zone attention unavailable")
		}
		codeRows, err := conn.QueryContext(ctx, `SELECT ad_code_id::text FROM ad_codes WHERE user_id=$1 AND is_active AND archived_at IS NULL`, user)
		if err != nil {
			return err
		}
		codes := []string{}
		for codeRows.Next() {
			var id string
			if err = codeRows.Scan(&id); err != nil {
				codeRows.Close()
				return err
			}
			codes = append(codes, id)
		}
		err = codeRows.Err()
		codeRows.Close()
		if err != nil {
			return err
		}
		known, missing, e := attentionInventory(ctx, provider, user, codes)
		if e != nil {
			return e
		}
		for code := range known {
			var active bool
			if e = conn.QueryRowContext(ctx, `SELECT is_active AND archived_at IS NULL FROM ad_codes WHERE ad_code_id=$1 AND user_id=$2`, code, user).Scan(&active); e != nil {
				return e
			}
			if !active {
				continue
			}
			var old []byte
			var notified bool
			e = conn.QueryRowContext(ctx, `SELECT missing,notified FROM publisher_attention WHERE ad_code_id=$1`, code).Scan(&old, &notified)
			if e != nil && !errorsNoRows(e) {
				return e
			}
			if len(missing[code]) > 0 {
				if !notified {
					body, _ := json.Marshal(missing[code])
					if e = s.userNotice(ctx, conn, user, "publisher:attention:"+code+":"+time.Now().UTC().Format("2006-01-02T15:04"), fmt.Sprintf("Publisher attention: code=%s", code)); e != nil {
						return e
					}
					if e = s.Bot.SendTextMessage(ctx, fmt.Sprintf("Publisher attention: user=%s code=%s sites=%s", user, code, body)); e != nil {
						return e
					}
					_, e = conn.ExecContext(ctx, `INSERT INTO publisher_attention(ad_code_id,missing,notified) VALUES($1,$2,true) ON CONFLICT(ad_code_id) DO UPDATE SET missing=EXCLUDED.missing,notified=true,updated_at=now()`, code, string(body))
					if e != nil {
						return e
					}
				}
			} else if notified {
				if e = s.userNotice(ctx, conn, user, "publisher:recovery:"+code+":"+time.Now().UTC().Format("2006-01-02T15:04"), fmt.Sprintf("Publisher recovery: code=%s", code)); e != nil {
					return e
				}
				if e = s.Bot.SendTextMessage(ctx, fmt.Sprintf("Publisher recovery: user=%s code=%s all missing sites recovered", user, code)); e != nil {
					return e
				}
				if _, e = conn.ExecContext(ctx, `DELETE FROM publisher_attention WHERE ad_code_id=$1`, code); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func errorsNoRows(e error) bool { return e == sql.ErrNoRows }

func (s *Service) userNotice(ctx context.Context, conn *sql.Conn, user, key, text string) error {
	// Existing notification API exposes these owner-bound notices with type other.
	_, e := conn.ExecContext(ctx, `INSERT INTO notifications(id,user_id,status,text,type) VALUES($1,$2,'active',$3,'other') ON CONFLICT(id) DO NOTHING`, uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String(), user, text)
	return e
}

func (s *Service) diagnostics(w http.ResponseWriter, r *http.Request) {
	code, e := s.ownerCode(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	limit, offset, e := pagination(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	report, e := s.zoneReport(r.Context(), auth.UserID(r), code.ID, limit, offset)
	if e != nil {
		reply(w, nil, e)
		return
	}
	ready := false
	if s.Redis != nil {
		v, err := s.Redis.Get(r.Context(), "twinbid:publisher:v1:{control}:ready").Result()
		ready = err == nil && v != ""
	}
	reply(w, map[string]any{"ad_code": code, "zones": report, "runtime_ready": ready}, nil)
}

// Consume bounded pages before acting on attention/recovery. An incomplete
// page sequence is an error, never evidence that all sites recovered.
func attentionInventory(ctx context.Context, provider ZoneReporter, user string, codes []string) (map[string]bool, map[string][]string, error) {
	known := map[string]bool{}
	missing := map[string][]string{}
	asof := time.Now().UTC()
	for offset := 0; ; offset += 500 {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		report, e := provider.PublisherZoneDiagnostics(ctx, user, codes, 500, offset, asof)
		if e != nil {
			return nil, nil, e
		}
		for _, z := range report.Rows {
			known[z.CodeID] = true
			if z.Status == "problem" {
				missing[z.CodeID] = append(missing[z.CodeID], z.Hostname)
			}
		}
		if len(report.Rows) < 500 {
			break
		}
	}
	return known, missing, nil
}
