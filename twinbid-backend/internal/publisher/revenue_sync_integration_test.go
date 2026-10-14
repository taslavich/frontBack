package publisher

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"twinbid-backend/internal/config"
)

type revenueFixture struct {
	mu      sync.Mutex
	totals  []RevenueTotal
	failure error
	calls   int
}

func (r *revenueFixture) PublisherQuery(context.Context, string, StatsRequest) (Report, error) {
	return Report{}, nil
}
func (r *revenueFixture) PublisherCumulativeRevenue(context.Context) ([]RevenueTotal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return append([]RevenueTotal{}, r.totals...), r.failure
}
func (r *revenueFixture) set(v []RevenueTotal) { r.mu.Lock(); defer r.mu.Unlock(); r.totals = v }
func TestCumulativeRevenueRestartLateCorrectionsAndPayoutDebt(t *testing.T) {
	s, h, user, other := fixture(t)
	source := &revenueFixture{totals: []RevenueTotal{{user, "100"}, {other, "35.123456789"}}}
	s.Reports = source
	for i := 0; i < 2; i++ {
		if e := s.SyncRevenue(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	b, e := s.Balance(context.Background(), user)
	if e != nil || b["balance"] != "100.00000000" || b["cumulative_earned"] != "100.00000000" {
		t.Fatal(b, e)
	}
	b, e = s.Balance(context.Background(), other)
	if e != nil || b["balance"] != "35.12345679" {
		t.Fatal(b, e)
	}
	restarted := New(s.DB, s.Redis, source, s.Config, config.BotConfig{})
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- restarted.SyncRevenue(context.Background()) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	// A completed payout permanently reduces balance, not cumulative earned.
	if _, e = s.DB.Exec(`INSERT INTO publisher_payouts(id,user_id,request_id,amount,method,wallet,status,transfer_reference) VALUES(gen_random_uuid(),$1,gen_random_uuid(),50,'usdt_erc20','wallet','paid','fixture')`, user); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(`UPDATE publisher_accounts SET balance=balance-50 WHERE user_id=$1`, user)
	if e = s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	b, _ = s.Balance(context.Background(), user)
	if b["balance"] != "50.00000000" {
		t.Fatal("restart reset paid funds", b)
	}
	source.set([]RevenueTotal{{user, "175"}, {other, "35.123456789"}})
	if e = s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	w := req(h, user, "POST", "/api/publisher/payouts", map[string]string{"amount": "75", "method": "usdt_erc20", "wallet": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "request_id": "10000000-0000-4000-8000-000000000099"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var pending string
	if e = s.DB.QueryRow(`SELECT id::text FROM publisher_payouts WHERE user_id=$1 AND status='pending'`, user).Scan(&pending); e != nil {
		t.Fatal(e)
	}
	source.set([]RevenueTotal{{user, "80"}, {other, "35.123456789"}})
	if e = s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	b, _ = s.Balance(context.Background(), user)
	if b["balance"] != "30.00000000" || b["reserved"] != "75.00000000" || b["available_for_payout"] != "0" {
		t.Fatal("negative delta lost", b)
	}
	if _, e = s.transition(context.Background(), pending, "", "paid", "tx"); e == nil {
		t.Fatal("unfunded reserve paid")
	}
	source.set([]RevenueTotal{})
	if e = s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	b, _ = s.Balance(context.Background(), user)
	if b["balance"] != "-50.00000000" {
		t.Fatal("debt erased", b)
	}
	source.mu.Lock()
	source.failure = errors.New("CH outage")
	source.mu.Unlock()
	if e = s.SyncRevenue(context.Background()); e == nil {
		t.Fatal("outage was accepted")
	}
	b, _ = s.Balance(context.Background(), user)
	if b["balance"] != "-50.00000000" || s.RevenueStatus(context.Background()) != "agg_stats_unavailable" {
		t.Fatal("outage changed money/readiness", b)
	}
	source.mu.Lock()
	source.failure = nil
	source.mu.Unlock()
	source.set([]RevenueTotal{{user, "120"}})
	if e = s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	b, _ = s.Balance(context.Background(), user)
	if b["balance"] != "70.00000000" {
		t.Fatal(b)
	}
	if w := req(h, user, "POST", "/api/internal/publisher/financial-events", map[string]string{}); w.Code != http.StatusNotFound {
		t.Fatal(w.Code)
	}
}
func TestRevenueInvalidOwnerAndAmountRollBackWholeSnapshot(t *testing.T) {
	s, _, user, _ := fixture(t)
	source := &revenueFixture{totals: []RevenueTotal{{user, "10"}}}
	s.Reports = source
	if e := s.SyncRevenue(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []RevenueTotal{{"invalid", "9"}, {user, "NaN"}, {user, "999999999999999999999999999"}} {
		source.set([]RevenueTotal{bad})
		if e := s.SyncRevenue(context.Background()); e == nil {
			t.Fatal("invalid source accepted", bad)
		}
		var cumulative, balance string
		if e := s.DB.QueryRow(`SELECT c.cumulative::text,a.balance::text FROM publisher_revenue_checkpoints c JOIN publisher_accounts a USING(user_id) WHERE user_id=$1`, user).Scan(&cumulative, &balance); e != nil || cumulative != "10.00000000" || balance != "10.00000000" {
			t.Fatal(cumulative, balance, e)
		}
	}
}
