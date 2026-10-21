package stats

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"twinbid-backend/internal/publisher"
)

// Uses the actual RTB schema prepared by the cross-project integration harness.
// It never creates an alternate guessed schema and never touches another owner.
func TestPublisherFormatGroupingRealSchema(t *testing.T) {
	addr := os.Getenv("PUBLISHER_TEST_CLICKHOUSE_ADDR")
	dbName := os.Getenv("PUBLISHER_FORMAT_TEST_DATABASE")
	if addr == "" || dbName == "" {
		t.Skip("PUBLISHER_TEST_CLICKHOUSE_ADDR/PUBLISHER_FORMAT_TEST_DATABASE unset")
	}
	db, e := sql.Open("clickhouse", fmt.Sprintf("clickhouse://default@%s/%s", addr, dbName))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &ClickHouseRepository{db: db, table: dbName + ".agg_stats"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	owner := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Hour)
	for i, format := range []string{"POP", "BAN", "NAT", "IPP", "VID"} {
		seededClicks, seededRevenue := uint64(i+1), float64(i+1)/100
		if format == "POP" {
			seededClicks = 0
			seededRevenue = 0
		}
		if _, e = db.ExecContext(ctx, "INSERT INTO "+dbName+".agg_stats(win_user_id,event_date,event_hour,format,pub_user_id,site_id,ad_code_id,zone_id,impressions,clicks,ssp_revenue_views) VALUES('advertiser',?,?,?,?, 'site','code','zone',?,?,?)", now, now, format, owner, uint64(100*(i+1)), seededClicks, seededRevenue); e != nil {
			t.Fatal(e)
		}
		if _, e = db.ExecContext(ctx, "INSERT INTO "+dbName+".ortb(uuid,event_time,code,format,pub_user_id,ad_code_id,zone_id,site_id,request_id,logical_event_id) VALUES(?,?,200,?,?,'code','zone','site',?,?)", uuid.NewString(), time.Now().UTC().Add(-time.Second), format, owner, uuid.NewString(), uuid.NewString()); e != nil {
			t.Fatal(e)
		}
	}

	// POP money is produced by the actual RTB fact->registry->reconciliation chain,
	// not by the artificial aggregate money used to challenge unsupported formats.
	click, _ := uuid.NewV7()
	if _, e = db.ExecContext(ctx, "INSERT INTO "+dbName+".fact_clicks(uuid,clicks_uuid,event_date,event_hour,format,pub_user_id,site_id,ad_code_id,zone_id,win_user_id,win_final_price) VALUES(?,?,?,?,?,?,'site','code','zone','advertiser',10)", uuid.NewString(), click.String(), now, now, "POP", owner); e != nil {
		t.Fatal(e)
	}
	for _, view := range []string{"mv_publisher_pop_revenue_to_agg_stats", "mv_publisher_pop_revenue_checkpoint"} {
		for _, action := range []string{"REFRESH", "WAIT"} {
			if _, e = db.ExecContext(ctx, "SYSTEM "+action+" VIEW "+dbName+"."+view); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, action := range []string{"REFRESH", "WAIT"} {
		if _, e = db.ExecContext(ctx, "SYSTEM "+action+" VIEW "+dbName+".mv_publisher_zone_hourly"); e != nil {
			t.Fatal(e)
		}
	}
	q := publisher.StatsRequest{From: now.Add(5 * time.Minute), To: now.Add(45 * time.Minute), GroupBy: "format", Timezone: "Asia/Shanghai", Limit: 2}
	first, e := r.PublisherQuery(ctx, owner, q)
	if e != nil {
		t.Fatal(e)
	}
	if len(first.Rows) != 2 || first.Totals.Impressions != 1500 || first.Totals.Requests != 5 || math.Abs(first.Totals.CTR-1) > 1e-8 || first.Totals.Revenue != "0.01" || first.TimeResolution != "1h_utc" || !first.EffectiveFrom.Equal(now) || !first.EffectiveTo.Equal(now.Add(time.Hour)) {
		t.Fatal(first)
	}
	q.Offset = 2
	q.Limit = 5
	rest, e := r.PublisherQuery(ctx, owner, q)
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, row := range append(first.Rows, rest.Rows...) {
		found[row.Bucket] = true
		if row.Bucket != "POP" && row.Revenue != "0" {
			t.Fatal("unsupported payable format", row)
		}
		if row.Requests != 1 || row.Fills != 1 || row.Impressions == 0 {
			t.Fatal(row)
		}
	}
	if len(found) != 5 {
		t.Fatal(found)
	}
	foreign, e := r.PublisherQuery(ctx, uuid.NewString(), q)
	if e != nil || foreign.Totals.Requests != 0 || foreign.Totals.Revenue != "0" {
		t.Fatal("owner leak", foreign, e)
	}
	// UI statistics and real cumulative income agree, with all other SSP owners
	// excluded. Non-POP input money cannot silently become payable MVP earnings.
	totals, err := r.PublisherCumulativeRevenue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, v := range totals {
		if v.UserID == owner {
			matched = true
			if v.Amount != "0.01" {
				t.Fatal(v)
			}
		}
	}
	if !matched {
		t.Fatal("owner missing")
	}
	// Financial producer remains POP-only until Publisher formats are supported.
	sql, e := publisherCumulativeQuery(dbName + ".agg_stats")
	if e != nil || !strings.Contains(sql, "format='POP'") {
		t.Fatal(sql, e)
	}
}
