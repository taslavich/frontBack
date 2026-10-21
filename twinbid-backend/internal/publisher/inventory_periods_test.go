package publisher

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

type inventoryPeriodReporter struct{ queries []StatsRequest }

func (*inventoryPeriodReporter) PublisherInventory(context.Context, string, string, string, string, int, int) ([]Row, error) {
	return []Row{{Bucket: "site"}}, nil
}
func (p *inventoryPeriodReporter) PublisherQuery(_ context.Context, _ string, q StatsRequest) (Report, error) {
	p.queries = append(p.queries, q)
	money := "1"
	if q.LifetimeRevenueOnly {
		money = "456"
	}
	return Report{Rows: []Row{{Bucket: "site", Metrics: Metrics{Impressions: 99, Clicks: 4, Revenue: money}}}}, nil
}
func TestInventoryMixedPeriodsPreserveAdvertisingAndLifetime(t *testing.T) {
	db := sql.OpenDB(patchTrafficConnector{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return nil, errors.New("test source state unavailable")
	}})
	defer db.Close()
	p := &inventoryPeriodReporter{}
	s := Service{DB: db, Reports: p}
	w := httptest.NewRecorder()
	s.inventory(w, httptest.NewRequest("GET", "/api/publisher/sites", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v struct {
		Rows         []Row  `json:"rows"`
		MetricPeriod string `json:"metric_period"`
		Scope        string `json:"metric_period_scope"`
		Requests     string `json:"request_metric_period"`
		Source       string `json:"request_metric_source"`
		Revenue      string `json:"revenue_period"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &struct {
		Data any `json:"data"`
	}{Data: &v}); e != nil {
		t.Fatal(e)
	}
	if v.MetricPeriod != "last_6_months" || v.Scope != "impressions_clicks_ctr" || v.Requests != "last_7_days" || v.Source != "publisher_zone_hourly" || v.Revenue != "lifetime" {
		t.Fatal(w.Body.String())
	}
	if len(p.queries) != 2 || p.queries[0].From.After(time.Now().Add(-170*24*time.Hour)) || !p.queries[1].LifetimeRevenueOnly {
		t.Fatal(p.queries)
	}
	if len(v.Rows) != 1 || v.Rows[0].Impressions != 99 || v.Rows[0].Clicks != 4 || v.Rows[0].Revenue != "456" || v.Rows[0].MetricsStatus != "mixed_periods" {
		t.Fatal(v.Rows)
	}
}
