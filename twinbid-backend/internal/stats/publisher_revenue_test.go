package stats

import (
	"strings"
	"testing"
)

func TestPublisherCumulativeOnlyUsesSharedPOPRevenue(t *testing.T) {
	q, e := publisherCumulativeQuery("custom_db.agg_stats")
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"custom_db.agg_stats", "format='POP'", "pub_user_id!=''", "sum(ssp_revenue_views)"} {
		if !strings.Contains(q, field) {
			t.Fatal(field, q)
		}
	}
	for _, forbidden := range []string{"fact_impressions", "pop_recovery_events", "win_dsp_price", "spend_views_table"} {
		if strings.Contains(q, forbidden) {
			t.Fatal(q)
		}
	}
	if _, e = publisherCumulativeQuery("agg_stats;DROP TABLE users"); e == nil {
		t.Fatal("unsafe configured name")
	}
	table, e := publisherCompanionTable("custom_db.agg_stats", "publisher_inventory")
	if e != nil || table != "custom_db.publisher_inventory" {
		t.Fatal(table, e)
	}
}
