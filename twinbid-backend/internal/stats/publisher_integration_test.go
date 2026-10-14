package stats

import (
	"context"
	"os"
	"testing"
	"time"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/publisher"
)

func TestPublisherReportsAgainstRealClickHouse(t *testing.T) {
	addr := os.Getenv("PUBLISHER_TEST_CLICKHOUSE_ADDR")
	if addr == "" {
		t.Skip("PUBLISHER_TEST_CLICKHOUSE_ADDR unset")
	}
	r, e := NewClickHouseRepository(context.Background(), config.ClickHouseConfig{Addr: addr, Database: "ads", Username: "default"})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if _, e = r.db.ExecContext(context.Background(), `INSERT INTO ortb(uuid,event_time,code,format,typic,pub_user_id,ad_code_id,zone_id,site_id,site_domain) VALUES('00000000-0000-0000-0000-000000000099',now64(3)-INTERVAL 365 DAY,204,'POP','MAINSTREAM','publisher','oldcode','oldzone','oldsite','old.example.test')`); e != nil {
		t.Fatal(e)
	}
	service := &Service{repo: r}
	inventory, e := service.PublisherInventory(context.Background(), "publisher", "site", "", "", 100, 0)
	if e != nil || len(inventory) != 2 {
		t.Fatal("lifetime discovery lost old site", inventory, e)
	}
	foreign, e := service.PublisherInventory(context.Background(), "another-publisher", "site", "oldsite", "", 100, 0)
	if e != nil || len(foreign) != 0 {
		t.Fatal("foreign lifetime inventory leaked", foreign, e)
	}
	for _, g := range []string{"date", "hour", "country", "device", "os", "browser", "site", "zone", "ad_code"} {
		v, e := r.PublisherQuery(context.Background(), "publisher", publisher.StatsRequest{From: time.Now().UTC().Add(-24 * time.Hour), To: time.Now().UTC(), GroupBy: g, Timezone: "UTC", Limit: 100})
		if e != nil {
			t.Fatal(g, e)
		}
		if v.Totals.Requests != 1 || v.Totals.Impressions != 1 {
			t.Fatal(g, v)
		}
	}
	filtered, e := r.PublisherQuery(context.Background(), "publisher", publisher.StatsRequest{From: time.Now().UTC().Add(-24 * time.Hour), To: time.Now().UTC(), GroupBy: "date", Timezone: "UTC", Limit: 100, Countries: []string{"US"}})
	if e != nil || filtered.Totals.Requests != 0 || filtered.Totals.Impressions != 0 {
		t.Fatal("filters not applied", filtered, e)
	}
	v, e := r.PublisherQuery(context.Background(), "another-publisher", publisher.StatsRequest{From: time.Now().UTC().Add(-24 * time.Hour), To: time.Now().UTC(), GroupBy: "site", Timezone: "UTC", Limit: 100, SiteID: "site"})
	if e != nil || len(v.Rows) != 0 || v.Totals.Requests != 0 || v.Totals.Impressions != 0 {
		t.Fatal("foreign source leaked", v, e)
	}
}
