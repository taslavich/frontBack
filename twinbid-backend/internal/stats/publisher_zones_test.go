package stats

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"os"
	"testing"
	"time"
)

func TestZoneCoverage(t *testing.T) {
	end := time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	beats := []time.Time{}
	for i := 0; i <= 12; i++ {
		beats = append(beats, start.Add(time.Duration(i)*5*time.Minute))
	}
	if !zoneCoverageReady(beats, time.Time{}, start, end) {
		t.Fatal("healthy chain rejected")
	}
	if zoneCoverageReady(nil, time.Time{}, start, end) {
		t.Fatal("empty accepted")
	}
	if zoneCoverageReady(beats[:8], time.Time{}, start, end) {
		t.Fatal("stale accepted")
	}
	gap := append(append([]time.Time{}, beats[:2]...), beats[6:]...)
	if zoneCoverageReady(gap, time.Time{}, start, end) {
		t.Fatal("gap accepted")
	}
	if !zoneCoverageReady(beats[10:], end.Add(-10*time.Minute), start, end) {
		t.Fatal("bootstrap rejected")
	}
}

// The integration runner prepares this database with CreateDB from the RTB
// project. These tests query its actual schema instead of a test-only variant.
func TestPublisherZonesRealSchema(t *testing.T) {
	addr, name := os.Getenv("PUBLISHER_TEST_CLICKHOUSE_ADDR"), os.Getenv("PUBLISHER_FORMAT_TEST_DATABASE")
	if addr == "" || name == "" {
		t.Skip("actual RTB fixture database not configured")
	}
	db, e := sql.Open("clickhouse", fmt.Sprintf("clickhouse://default@%s/%s", addr, name))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &ClickHouseRepository{db: db, table: name + ".agg_stats"}
	ctx := context.Background()
	user := uuid.NewString()
	must := func(q string, a ...any) {
		if _, e := db.ExecContext(ctx, q, a...); e != nil {
			t.Fatal(e, q)
		}
	}
	for _, z := range []string{"ok", "empty", "mixed", "error", "silent"} {
		must("INSERT INTO "+name+".publisher_inventory(pub_user_id,ad_code_id,zone_id,site_id,hostname,first_request_at,last_request_at) SELECT ?,'trusted',?,'site','example.test',now64(3,'UTC')-INTERVAL 2 HOUR,now64(3,'UTC')-INTERVAL 2 HOUR", user, z)
	}
	for _, item := range []struct {
		z    string
		code int
	}{{"ok", 200}, {"empty", 204}, {"mixed", 200}, {"mixed", 500}, {"error", 403}, {"error", 503}} {
		must("INSERT INTO "+name+".ortb(uuid,logical_event_id,event_time,format,code,pub_user_id,ad_code_id,zone_id,site_id,site_domain) SELECT toUUID(?),?,now64(3,'UTC')-INTERVAL 2 MINUTE,'POP',?,?,'trusted',?,'site','example.test'", uuid.NewString(), uuid.NewString(), item.code, user, item.z)
	}
	for _, action := range []string{"REFRESH", "WAIT"} {
		must("SYSTEM " + action + " VIEW " + name + ".mv_publisher_zone_hourly")
	}
	end := time.Now().UTC().Add(time.Second)
	report, e := r.PublisherZoneDiagnostics(ctx, user, []string{"trusted", "not-yet-detected"}, 2, 0, end)
	if e != nil || len(report.Rows) != 2 || report.ActiveZones != 3 || report.ProblemZones != 2 || report.CodesWithoutZones != 1 || len(report.UndiscoveredCodes) != 1 || report.UndiscoveredCodes[0].CodeID != "not-yet-detected" || report.UndiscoveredCodes[0].Status != "no_zones_detected" {
		t.Fatal(report, e)
	}
	second, e := r.PublisherZoneDiagnostics(ctx, user, []string{"trusted"}, 500, 2, end)
	if e != nil {
		t.Fatal(e)
	}
	rows := append(report.Rows, second.Rows...)
	if len(rows) != 5 {
		t.Fatal(rows)
	}
	for _, z := range rows {
		if (z.ZoneID == "empty" || z.ZoneID == "mixed" || z.ZoneID == "ok") && z.Status != "active" {
			t.Fatal(z)
		}
		if z.ZoneID == "mixed" && z.Requests5xx != 1 {
			t.Fatal(z)
		}
		if (z.ZoneID == "error" || z.ZoneID == "silent") && z.Status != "problem" {
			t.Fatal(z)
		}
		if z.ZoneID == "silent" && z.LastRequestAt == nil {
			t.Fatal("lost registry last request")
		}
	}
	for _, tc := range []struct {
		user  string
		codes []string
	}{{uuid.NewString(), []string{"trusted"}}, {user, []string{"unowned"}}, {user, nil}} {
		v, e := r.PublisherZoneDiagnostics(ctx, tc.user, tc.codes, 500, 0, end)
		if e != nil || len(v.Rows) != 0 || v.ActiveZones != 0 || v.ProblemZones != 0 {
			t.Fatal("owner/code leak", v, e)
		}
	}
	empty, e := r.PublisherZoneDiagnostics(ctx, user, []string{"z-missing", "a-missing", "z-missing"}, 1, 1, end)
	if e != nil || len(empty.Rows) != 0 || empty.ActiveZones != 0 || empty.ProblemZones != 0 || empty.CodesWithoutZones != 2 || len(empty.UndiscoveredCodes) != 1 || empty.UndiscoveredCodes[0].CodeID != "z-missing" {
		t.Fatal("code-level absence or pagination", empty, e)
	}
	// Stale service cannot report success or substitute zeros.
	if _, e = r.PublisherZoneDiagnostics(ctx, user, []string{"trusted"}, 500, 0, end.Add(2*time.Hour)); e == nil {
		t.Fatal("stale diagnostics accepted")
	}
}
