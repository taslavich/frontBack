package stats

import (
	"strings"
	"testing"
)

const (
	testUserID     = "11111111-1111-1111-1111-111111111111"
	testCampaignID = "22222222-2222-2222-2222-222222222222"
	testCreativeID = "33333333-3333-3333-3333-333333333333"
)

func TestBuildStatsQueriesHourWithFilters(t *testing.T) {
	req := QueryRequest{
		From:        "2026-05-01",
		To:          "2026-05-06",
		GroupBy:     GroupByHour,
		CampaignIDs: []string{testCampaignID},
		CreativeIDs: []string{testCreativeID},
		Filters: map[string][]string{
			"country":     {"DE", "US"},
			"os":          {"Android"},
			"browser":     {},
			"device_type": {"mobile"},
		},
	}

	rowsPlan, totalsPlan, err := buildStatsQueries(testUserID, req, "ads.agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, rowsPlan.SQL, "formatDateTime(toStartOfHour(event_hour), '%F %H:00', 'UTC') AS bucket")
	mustContain(t, rowsPlan.SQL, "FROM ads.agg_stats")
	mustContain(t, rowsPlan.SQL, "user_id = toUUID(?)")
	mustContain(t, rowsPlan.SQL, "campaign_id IN (toUUID(?))")
	mustContain(t, rowsPlan.SQL, "creative_id IN (toUUID(?))")
	mustContain(t, rowsPlan.SQL, "geo IN (?,?)")
	mustContain(t, rowsPlan.SQL, "os IN (?)")
	mustContain(t, rowsPlan.SQL, "device_type IN (?)")
	mustContain(t, rowsPlan.SQL, "GROUP BY toStartOfHour(event_hour)")
	mustContain(t, rowsPlan.SQL, "ORDER BY bucket")
	mustContain(t, rowsPlan.SQL, "lowerUTF8(ifNull(format, '')) IN ('ban', 'nat'), spend_views_table")
	mustContain(t, rowsPlan.SQL, "lowerUTF8(ifNull(format, '')) IN ('ipp', 'pop'), spend_clicks_table")

	if len(rowsPlan.Args) != 9 {
		t.Fatalf("expected 9 args, got %d: %#v", len(rowsPlan.Args), rowsPlan.Args)
	}
	if rowsPlan.Args[0] != testUserID || rowsPlan.Args[1] != "2026-05-01" || rowsPlan.Args[2] != "2026-05-06" {
		t.Fatalf("unexpected base args: %#v", rowsPlan.Args[:3])
	}
	if len(totalsPlan.Args) != len(rowsPlan.Args) {
		t.Fatalf("totals args must match rows args")
	}
	if strings.Contains(totalsPlan.SQL, "GROUP BY") {
		t.Fatalf("totals query must not contain GROUP BY: %s", totalsPlan.SQL)
	}
}

func TestBuildStatsQueriesCampaignHasNoOrderBy(t *testing.T) {
	req := QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByCampaign}

	rowsPlan, _, err := buildStatsQueries(testUserID, req, "agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, rowsPlan.SQL, "toString(campaign_id) AS bucket")
	mustContain(t, rowsPlan.SQL, "GROUP BY campaign_id")
	mustNotContain(t, rowsPlan.SQL, "ORDER BY")
}

func TestBuildStatsQueriesDateHasOrderByDate(t *testing.T) {
	req := QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByDate}

	rowsPlan, _, err := buildStatsQueries(testUserID, req, "agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, rowsPlan.SQL, "toString(event_date) AS bucket")
	mustContain(t, rowsPlan.SQL, "GROUP BY event_date")
	mustContain(t, rowsPlan.SQL, "ORDER BY bucket")
}

func TestBuildStatsQueriesInvalidGroupBy(t *testing.T) {
	req := QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupBy("format")}

	_, _, err := buildStatsQueries(testUserID, req, "agg_stats")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid group_by") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStatsQueriesInvalidFilter(t *testing.T) {
	req := QueryRequest{
		From:    "2026-05-01",
		To:      "2026-05-06",
		GroupBy: GroupByCountry,
		Filters: map[string][]string{"site_id": {"abc"}},
	}

	_, _, err := buildStatsQueries(testUserID, req, "agg_stats")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid filter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStatsQueriesInvalidUUID(t *testing.T) {
	req := QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByCampaign}

	_, _, err := buildStatsQueries("bad-user-id", req, "agg_stats")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid user_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStatsQueriesInvalidDateRange(t *testing.T) {
	req := QueryRequest{From: "2026-05-06", To: "2026-05-01", GroupBy: GroupByCampaign}

	_, _, err := buildStatsQueries(testUserID, req, "agg_stats")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "from must be <= to") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeTableRejectsUnsafeName(t *testing.T) {
	_, err := normalizeTable("agg_stats; DROP TABLE users")
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustContain(t *testing.T, s string, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected SQL to contain %q\nSQL:\n%s", sub, s)
	}
}

func mustNotContain(t *testing.T, s string, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Fatalf("expected SQL not to contain %q\nSQL:\n%s", sub, s)
	}
}
