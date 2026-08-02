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
