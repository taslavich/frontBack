package stats

import (
	"strings"
	"testing"
)

func TestBuildCumulativeSpendQuery(t *testing.T) {
	query, err := buildCumulativeSpendQuery("ads.agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(query, "toUUIDOrNull") {
		t.Fatalf("cumulative spend query must not use ClickHouse permissive UUID conversion:\n%s", query)
	}
	for _, fragment := range []string{
		"length(entity_id) = 36",
		"match(entity_id",
		canonicalUUIDClickHouseRegexp,
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("cumulative spend query missing strict UUID filter %q:\n%s", fragment, query)
		}
	}
	for _, fragment := range []string{
		"FROM ads.agg_stats",
		"GROUP BY GROUPING SETS",
		"(win_user_id)",
		"(win_cid)",
		"spend_views_table",
		"spend_clicks_table",
		"('ban', 'nat', 'pop')",
		"= 'ipp'",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, query)
		}
	}
}

func TestBuildCumulativeSpendQueryRejectsUnsafeTable(t *testing.T) {
	if _, err := buildCumulativeSpendQuery("agg_stats; DROP TABLE users"); err == nil {
		t.Fatal("expected unsafe table name to be rejected")
	}
}

func TestBuildPOPRecoveredSpendQuery(t *testing.T) {
	query, err := buildPOPRecoveredSpendQuery("ads.agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(query, "toUUIDOrNull") {
		t.Fatalf("POP recovery query must not rely on permissive UUID conversion:\n%s", query)
	}
	for _, fragment := range []string{
		"length(trimBoth(win_user_id)) = 36",
		"match(trimBoth(win_user_id)",
		"length(trimBoth(win_cid)) = 36",
		"match(trimBoth(win_cid)",
		canonicalUUIDClickHouseRegexp,
		"FROM ads.agg_stats",
		"sum(pop_recovered_spend)",
		"trimBoth(win_user_id) AS user_id",
		"trimBoth(win_cid) AS campaign_id",
		"pop_recovered_spend > 0",
		"GROUP BY",
		"win_user_id",
		"win_cid",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("POP recovery query does not contain %q:\n%s", fragment, query)
		}
	}
	if strings.Contains(strings.ToUpper(query), "UPDATE ") || strings.Contains(strings.ToUpper(query), "ALTER ") {
		t.Fatalf("spend sync must only filter/select raw win_cid, never rewrite storage semantics:\n%s", query)
	}
}

func TestBuildPOPRecoveredSpendQueryRejectsUnsafeTable(t *testing.T) {
	if _, err := buildPOPRecoveredSpendQuery("agg_stats; DROP TABLE users"); err == nil {
		t.Fatal("expected unsafe table name to be rejected")
	}
}

func TestBuildFilteredNonCanonicalUUIDLikeCampaignQuery(t *testing.T) {
	query, err := buildFilteredNonCanonicalUUIDLikeCampaignQuery("ads.agg_stats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, fragment := range []string{
		"FROM ads.agg_stats",
		"trimBoth(win_cid)",
		"isNotNull(toUUIDOrNull(trimBoth(win_cid)))",
		"NOT (",
		canonicalUUIDClickHouseRegexp,
		"LIMIT 5",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("diagnostic query missing %q:\n%s", fragment, query)
		}
	}

	upper := strings.ToUpper(query)
	if strings.Contains(upper, "UPDATE ") || strings.Contains(upper, "ALTER ") {
		t.Fatalf("diagnostic query must be read-only:\n%s", query)
	}
}

func TestBuildFilteredNonCanonicalUUIDLikeCampaignQueryRejectsUnsafeTable(t *testing.T) {
	if _, err := buildFilteredNonCanonicalUUIDLikeCampaignQuery("agg_stats; DROP TABLE users"); err == nil {
		t.Fatal("expected unsafe table name to be rejected")
	}
}
