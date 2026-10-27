package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/db"
	c "twinbid-backend/internal/pubcontract"
	"twinbid-backend/internal/publisherconfig"
)

func fixture(t *testing.T) (*Service, http.Handler, string, string) {
	t.Helper()
	dsn := os.Getenv("PUBLISHER_TEST_POSTGRES_DSN")
	addr := os.Getenv("PUB_TEST_REDIS_ADDR")
	if dsn == "" || addr == "" {
		t.Skip("SQL/Redis integration ENV unset")
	}
	pg, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	pg.SetMaxOpenConns(8)
	pg.SetConnMaxLifetime(time.Minute)
	t.Cleanup(func() { pg.Close() })
	if e = db.EnsurePublisherSchema(context.Background(), pg); e != nil {
		t.Fatal(e)
	}
	user, other := uuid.NewString(), uuid.NewString()
	if _, e = pg.Exec(`INSERT INTO users(id) VALUES($1),($2)`, user, other); e != nil {
		t.Fatal(e)
	}
	r := redis.NewClient(&redis.Options{Addr: addr, DB: 14})
	t.Cleanup(func() { r.Close() })
	if e = r.FlushDB(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
	_, e = pg.Exec(`UPDATE publisher_control SET epoch='',state='blocked' WHERE id=1`)
	if e != nil {
		t.Fatal(e)
	}
	s := New(pg, r, nil, publisherconfig.Config{SyncInterval: time.Second}, config.BotConfig{})
	as := auth.NewService(auth.NewRepository(pg), config.JWTConfig{Secret: "test"}, config.SMTPConfig{})
	router := chi.NewRouter()
	router.Use(auth.Middleware(as))
	s.Routes(router)
	return s, router, user, other
}
func req(h http.Handler, user, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	token, _, _ := auth.GenerateJWT("test", user, "test@example.test", "access", time.Minute)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestCRUDOwnershipVersionSyncAndRedisReset(t *testing.T) {
	s, h, user, other := fixture(t)
	a, e := s.insertCode(context.Background(), user, c.Code{Name: "A", Format: "popunder", TrafficType: "mainstream", Device: "all", TriggerClick: 2, FrequencyMinutes: .1})
	if e != nil {
		t.Fatal(e)
	}
	if w := req(h, other, "GET", "/api/publisher/ad-codes/"+a.ID, nil); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+a.ID, map[string]any{"is_active": false, "version": 1, "traffic_type": "mainstream"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+a.ID, map[string]any{"name": "stale", "version": 1, "traffic_type": "mainstream"}); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := req(h, other, "GET", "/api/publisher/sync-health", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"backlog":0`) {
		t.Fatal("sync health owner isolation", w.Code, w.Body.String())
	}
	if e = s.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	var first, second string
	s.DB.QueryRow(`SELECT epoch FROM publisher_control WHERE id=1`).Scan(&first)
	b, e := s.Redis.HGet(context.Background(), c.CodeKey(a.ShortCode), "data").Bytes()
	if e != nil {
		t.Fatal(e)
	}
	var code c.Code
	json.Unmarshal(b, &code)
	if code.Active || code.Version != 2 {
		t.Fatal("stale Redis config", code)
	}
	if s.Redis.Get(context.Background(), c.ReadyKey).Val() != first {
		t.Fatal("publisher did not become ready after initial sync")
	}
	var initState string
	if e = s.DB.QueryRow(`SELECT state FROM publisher_control WHERE id=1`).Scan(&initState); e != nil || initState != "ready" {
		t.Fatal("publisher not automatically unblocked", initState, e)
	}
	s.Redis.FlushDB(context.Background())
	if e = s.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	s.DB.QueryRow(`SELECT epoch FROM publisher_control WHERE id=1`).Scan(&second)
	if first == second || s.Redis.Get(context.Background(), c.ReadyKey).Val() != second {
		t.Fatal("publisher did not automatically rehydrate after Redis data loss")
	}
	// Loss of old guard keys or Redis server run_id must not block traffic.
	s.Redis.Del(context.Background(), c.GuardKeyForSlot(0))
	if e = s.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	var third string
	s.DB.QueryRow(`SELECT epoch FROM publisher_control WHERE id=1`).Scan(&third)
	if third != second {
		t.Fatal("guard loss rotated epoch without dynamic state loss")
	}
	if w := req(h, user, "DELETE", "/api/publisher/ad-codes/"+a.ID, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = s.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	b, e = s.Redis.HGet(context.Background(), c.CodeKey(a.ShortCode), "data").Bytes()
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &code)
	if !code.Archived || code.Active {
		t.Fatal("archive did not arrive")
	}
}
func TestFinancialRevisionAndPayoutReserveRace(t *testing.T) {
	s, h, user, _ := fixture(t)
	source := &revenueFixture{totals: []RevenueTotal{{UserID: user, Amount: "100"}}}
	s.Reports = source
	for i := 0; i < 2; i++ {
		if e := s.SyncRevenue(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	source.set([]RevenueTotal{{UserID: user, Amount: "101"}})
	if e := s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	var e error
	wallet := "0x" + strings.Repeat("a", 40)
	var wg sync.WaitGroup
	status := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := req(h, user, "POST", "/api/publisher/payouts", map[string]string{"amount": "75", "method": "usdt_erc20", "wallet": wallet, "request_id": uuid.NewString()})
			status <- w.Code
		}()
	}
	wg.Wait()
	close(status)
	success := 0
	for v := range status {
		if v == 200 {
			success++
		} else if v != 409 {
			t.Fatal(v)
		}
	}
	if success != 1 {
		t.Fatal("double reserve", success)
	}
	b, e := s.Balance(context.Background(), user)
	if e != nil || b["available_for_payout"] != "26" {
		t.Fatal(b, e)
	}
	var id string
	s.DB.QueryRow(`SELECT id::text FROM publisher_payouts WHERE user_id=$1`, user).Scan(&id)
	for i := 0; i < 2; i++ {
		if _, e = s.transition(context.Background(), id, "", "canceled", ""); e != nil {
			t.Fatal(e)
		}
	}
	b, _ = s.Balance(context.Background(), user)
	if b["available_for_payout"] != "101" {
		t.Fatal("reserve released twice", b)
	}
	if _, e = s.transition(context.Background(), id, "", "paid", "tx"); e == nil {
		t.Fatal("canceled payout became paid")
	}
}
func TestImmutableCodeIdentity(t *testing.T) {
	s, _, user, other := fixture(t)
	code, e := s.insertCode(context.Background(), user, c.Code{Name: "A", Format: "popunder", TrafficType: "mainstream", Device: "all", TriggerClick: 1, FrequencyMinutes: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`UPDATE ad_codes SET user_id=$2 WHERE ad_code_id=$1`, code.ID, other); e == nil {
		t.Fatal("owner reassigned")
	}
}

var _ = fmt.Sprintf
