package publisher

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/config"
	c "twinbid-backend/internal/pubcontract"
)

// SQL test double exercises real handlers/synchronizer and bound parameters;
// it deliberately does not claim to execute PostgreSQL SQL or migrations.
type siteBlockRow struct {
	blocked         bool
	reason          string
	version, synced int64
	epoch           string
}
type siteBlockSQL struct {
	mu        sync.Mutex
	rows      map[string]siteBlockRow
	failWrite bool
	epoch     string
	status    string
}
type siteBlockConnector struct{ state *siteBlockSQL }
type siteBlockDriver struct{ state *siteBlockSQL }
type siteBlockConn struct{ state *siteBlockSQL }
type siteBlockRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (d siteBlockDriver) Open(string) (driver.Conn, error) { return &siteBlockConn{d.state}, nil }
func (d siteBlockConnector) Connect(context.Context) (driver.Conn, error) {
	return &siteBlockConn{d.state}, nil
}
func (d siteBlockConnector) Driver() driver.Driver { return siteBlockDriver{d.state} }
func (d *siteBlockConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepare")
}
func (d *siteBlockConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("unexpected transaction") }
func (d *siteBlockConn) Close() error              { return nil }
func (r *siteBlockRows) Columns() []string         { return r.columns }
func (r *siteBlockRows) Close() error              { return nil }
func (r *siteBlockRows) Next(dest []driver.Value) error {
	if r.index == len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func (d *siteBlockConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	switch {
	case strings.HasPrefix(query, "INSERT INTO publisher_site_blocks(site_id,blocked,reason)"):
		if d.state.failWrite {
			return nil, fmt.Errorf("test write failure")
		}
		id := args[0].Value.(string)
		row := d.state.rows[id]
		row.blocked = args[1].Value.(bool)
		row.reason = args[2].Value.(string)
		row.version++
		d.state.rows[id] = row
	case strings.HasPrefix(query, "UPDATE publisher_site_blocks SET synced_version="):
		id := args[0].Value.(string)
		row := d.state.rows[id]
		if row.version == args[1].Value.(int64) {
			row.synced = args[1].Value.(int64)
			row.epoch = args[2].Value.(string)
			d.state.rows[id] = row
		}
	case query == "UPDATE publisher_control SET epoch=$1,state='ready' WHERE id=1":
		d.state.epoch, d.state.status = args[0].Value.(string), "ready"
	case query == "UPDATE publisher_control SET state='ready' WHERE id=1":
		d.state.status = "ready"
	case strings.HasPrefix(query, "SELECT pg_advisory_unlock("):
	default:
		return nil, fmt.Errorf("unexpected SQL: %s", query)
	}
	return driver.RowsAffected(1), nil
}

func (d *siteBlockConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	switch {
	case strings.HasPrefix(query, "SELECT pg_try_advisory_lock("):
		return &siteBlockRows{columns: []string{"locked"}, values: [][]driver.Value{{true}}}, nil
	case query == "SELECT epoch,state FROM publisher_control WHERE id=1":
		return &siteBlockRows{columns: []string{"epoch", "state"}, values: [][]driver.Value{{d.state.epoch, d.state.status}}}, nil
	case strings.HasPrefix(query, "SELECT "+codeColumns+" FROM ad_codes "):
		return &siteBlockRows{columns: []string{"empty"}}, nil
	case strings.HasPrefix(query, "SELECT site_id,blocked,reason,version FROM publisher_site_blocks "):
		rows := &siteBlockRows{columns: []string{"site_id", "blocked", "reason", "version"}}
		for id, row := range d.state.rows {
			if row.synced < row.version || row.epoch != d.state.epoch {
				rows.values = append(rows.values, []driver.Value{id, row.blocked, row.reason, row.version})
			}
		}
		return rows, nil
	case strings.HasPrefix(query, "SELECT (SELECT count(*) FROM ad_codes WHERE (version>synced_version OR synced_epoch<>c.epoch)") && strings.Contains(query, "publisher_site_blocks"):
		var pending int64
		if args[0].Value == "" {
			for _, row := range d.state.rows {
				if row.synced < row.version || row.epoch != d.state.epoch {
					pending++
				}
			}
		}
		return &siteBlockRows{columns: []string{"pending", "state", "epoch"}, values: [][]driver.Value{{pending, d.state.status, d.state.epoch}}}, nil
	case strings.HasPrefix(query, "SELECT (SELECT count(*) FROM ad_codes ") && strings.Contains(query, "publisher_site_blocks"):
		var pending int64
		for _, row := range d.state.rows {
			if row.synced < row.version || row.epoch != d.state.epoch {
				pending++
			}
		}
		return &siteBlockRows{columns: []string{"pending"}, values: [][]driver.Value{{pending}}}, nil
	default:
		return nil, fmt.Errorf("unexpected SQL: %s", query)
	}
}

func siteBlockHandler(t *testing.T) (*Service, http.Handler, *siteBlockSQL, string) {
	t.Helper()
	state := &siteBlockSQL{rows: map[string]siteBlockRow{}, epoch: "test-site-epoch", status: "ready"}
	db := sql.OpenDB(siteBlockConnector{state})
	t.Cleanup(func() { db.Close() })
	admin := uuid.NewString()
	s := &Service{DB: db, AdminID: admin}
	h := chi.NewRouter()
	h.Use(auth.Middleware(auth.NewService(nil, config.JWTConfig{Secret: "test"}, config.SMTPConfig{})))
	s.Routes(h)
	return s, h, state, admin
}

func TestSiteBlockAPIOnlyAcceptsReadyIDFromAdmin(t *testing.T) {
	s, h, state, admin := siteBlockHandler(t)
	const id = "U368FL4X4rW8"
	body := map[string]any{"site_id": id, "blocked": true, "reason": "test"}
	if w := req(h, uuid.NewString(), "PUT", "/api/publisher/admin/domain-block", body); w.Code != 403 {
		t.Fatal("non-admin accepted", w.Code)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("PUT", "/api/publisher/admin/domain-block", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal("JWT not required", w.Code)
	}
	for _, bad := range []map[string]any{
		{"hostname": "example.com", "blocked": true}, {"site_id": "example.com", "blocked": true},
		{"blocked": true}, {"site_id": strings.Repeat("a", 64), "blocked": true},
		{"site_id": "ABCDEFGHIJK_", "blocked": true}, {"site_id": " " + id, "blocked": true},
		{"site_id": "ABCDEFGHIJKя", "blocked": true}, {"site_id": nil, "blocked": true},
	} {
		if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", bad); w.Code != 400 {
			t.Fatal("invalid site accepted", bad, w.Code, w.Body.String())
		}
	}
	if len(state.rows) != 0 {
		t.Fatal("unauthorized/invalid requests wrote SQL")
	}
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 200 || !strings.Contains(w.Body.String(), `"site_id":"`+id+`"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if row := state.rows[id]; !row.blocked || row.version != 1 || len(state.rows) != 1 {
		t.Fatal("input Site ID was transformed", state.rows)
	}
	body["blocked"] = false
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if row := state.rows[id]; row.blocked || row.version != 2 {
		t.Fatal("unblock/version failed", row)
	}
	// Base62 is case-sensitive, including at the persistence boundary.
	body["site_id"] = strings.ToLower(id)
	body["blocked"] = true
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 200 || len(state.rows) != 2 {
		t.Fatal("case-sensitive IDs collapsed", w.Code, state.rows)
	}
	state.failWrite = true
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 503 {
		t.Fatal("SQL failure reported as success", w.Code, w.Body.String())
	}
	s.AdminID = ""
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 403 {
		t.Fatal("empty admin configuration accepted")
	}
}

func TestSiteBlockSyncWritesRawSiteIDAndUnblockToRedis(t *testing.T) {
	addr := os.Getenv("PUB_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("PUB_TEST_REDIS_ADDR unset")
	}
	s, h, state, admin := siteBlockHandler(t)
	ctx := context.Background()
	r := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	defer r.Close()
	s.Redis = r
	if err := r.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.Set(ctx, c.EpochKey, "test-site-epoch", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.HSet(ctx, c.SiteBlocksKey, "epoch", "test-site-epoch").Err(); err != nil {
		t.Fatal(err)
	}
	const id = "U368FL4X4rW8"
	for _, blocked := range []bool{true, false, true} {
		body := map[string]any{"site_id": id, "blocked": blocked, "reason": "admin decision"}
		if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", body); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if err := s.Sync(ctx); err != nil {
			t.Fatal(err)
		}
		want := "0"
		if blocked {
			want = "1"
		}
		if got := r.HGet(ctx, c.SiteBlocksKey, id).Val(); got != want {
			t.Fatal("policy does not use the exact Site ID", got, want)
		}
		data, err := r.HGet(ctx, c.BlockKey(id), "data").Bytes()
		if err != nil {
			t.Fatal(err)
		}
		var stored struct {
			SiteID   string `json:"site_id"`
			Hostname string `json:"hostname"`
			Blocked  bool   `json:"blocked"`
		}
		if err = json.Unmarshal(data, &stored); err != nil || stored.SiteID != id || stored.Hostname != "" || stored.Blocked != blocked {
			t.Fatal("Site ID was hashed or replaced", stored, err)
		}
		before := state.rows[id]
		if before.synced != before.version {
			t.Fatal("synchronized version not committed", before)
		}
		if err = s.Sync(ctx); err != nil {
			t.Fatal(err)
		}
		if after := state.rows[id]; after != before {
			t.Fatal("repeat sync modified the rule", before, after)
		}
	}
	if r.Exists(ctx, c.Prefix+"{control}:blocks").Val() != 0 {
		t.Fatal("legacy domain policy was recreated")
	}
}

func TestSiteBlockSyncPreservesFProj260RecoveryAndHealth(t *testing.T) {
	addr := os.Getenv("PUB_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("PUB_TEST_REDIS_ADDR unset")
	}
	s, h, state, admin := siteBlockHandler(t)
	ctx := context.Background()
	r := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	defer r.Close()
	s.Redis = r
	if err := r.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	const id = "U368FL4X4rW8"
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", map[string]any{"site_id": id, "blocked": true}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// The original FProj260 auto-recovery must still run when the new policy
	// is absent. A legacy domain policy cannot stand in for the Site ID policy.
	if err := r.Set(ctx, c.EpochKey, state.epoch, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.HSet(ctx, c.Prefix+"{control}:blocks", "epoch", state.epoch).Err(); err != nil {
		t.Fatal(err)
	}
	oldEpoch := state.epoch
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if state.epoch == oldEpoch || state.status != "ready" || r.HGet(ctx, c.SiteBlocksKey, id).Val() != "1" || r.HGet(ctx, c.SiteBlocksKey, "epoch").Val() != state.epoch {
		t.Fatal("new Site ID policy was not rehydrated", state)
	}
	if r.HGet(ctx, c.Prefix+"{control}:blocks", "epoch").Val() != oldEpoch {
		t.Fatal("legacy domain policy was modified")
	}
	// AOF-style restoration with matching epochs must keep the generation.
	for _, status := range []string{"ready", "blocked"} {
		state.status = status
		epoch := state.epoch
		if err := s.Sync(ctx); err != nil {
			t.Fatal(err)
		}
		if state.epoch != epoch || state.status != "ready" || r.Get(ctx, c.StateKey).Val() != "ready" || r.TTL(ctx, c.ReadyKey).Val() != -1 {
			t.Fatal("FProj260 restored-state/readiness behavior changed", status)
		}
	}
	checkHealth := func(owner string, ready bool, backlog int, epoch string) {
		t.Helper()
		w := req(h, owner, "GET", "/api/publisher/sync-health", nil)
		var response struct {
			Data struct {
				Ready   bool   `json:"ready"`
				Backlog int    `json:"backlog"`
				Epoch   string `json:"epoch"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		v := response.Data
		if err != nil || w.Code != 200 || v.Ready != ready || v.Backlog != backlog || v.Epoch != epoch {
			t.Fatal("health must use Site ID backlog and Redis epoch", w.Code, w.Body.String(), err)
		}
	}
	if err := r.Del(ctx, c.ReadyKey).Err(); err != nil {
		t.Fatal(err)
	}
	checkHealth(admin, true, 0, state.epoch)
	if w := req(h, admin, "PUT", "/api/publisher/admin/domain-block", map[string]any{"site_id": id, "blocked": false}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	checkHealth(admin, false, 1, state.epoch)
	checkHealth(uuid.NewString(), true, 0, "")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	checkHealth(admin, true, 0, state.epoch)
	if err := r.Set(ctx, c.EpochKey, "mismatch", 0).Err(); err != nil {
		t.Fatal(err)
	}
	checkHealth(admin, false, 0, state.epoch)
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if r.HGet(ctx, c.SiteBlocksKey, id).Val() != "0" || state.rows[id].epoch != state.epoch {
		t.Fatal("Redis reset did not replay the already synchronized unblock")
	}
	checkHealth(admin, true, 0, state.epoch)
}
