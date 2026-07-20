package stats

import (
	"strings"
	"testing"
)

func TestBuildCalculatorPlanMatchesFrontendContract(t *testing.T) {
	plan, err := buildCalculatorPlan(TrafficSegmentRequest{
		FormatType:     "banner",
		TrafficType:    "mixed",
		Country:        []string{"de", "FR", "DE"},
		CountryMode:    FilterModeInclude,
		Language:       []string{"DE"},
		LanguageMode:   FilterModeExclude,
		DeviceType:     []string{"Desktop"},
		DeviceTypeMode: FilterModeInclude,
		OS:             []string{"Windows"},
		OSMode:         FilterModeInclude,
		Browser:        []string{"Chrome"},
		BrowserMode:    FilterModeExclude,
	}, "ads.traffic_volume_hourly")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, plan.SQL, "event_hour >= toStartOfDay(now('UTC')) - INTERVAL 1 DAY")
	mustContain(t, plan.SQL, "event_hour < toStartOfDay(now('UTC'))")
	mustContain(t, plan.SQL, "toUInt64(ifNull(sum(requests), 0)) AS potential_impressions")
	mustContain(t, plan.SQL, "format = ?")
	mustContain(t, plan.SQL, "typic IN (?,?)")
	mustContain(t, plan.SQL, "geo IN (?,?)")
	mustContain(t, plan.SQL, "lang NOT IN (?)")
	mustContain(t, plan.SQL, "device IN (?)")
	mustContain(t, plan.SQL, "browser NOT IN (?,?,?,?,?,?,?,?)")

	want := []any{
		"BAN", "MAINSTREAM", "ADULT",
		"DE", "FR", "de", "desktop", "Windows",
		"Chrome", "Chromium", "Ubuntu Chromium", "Raspbian Chromium",
		"Kiwi Chrome", "Iron", "Comodo_Dragon", "JSChromeBrowser",
	}
	if len(plan.Args) != len(want) {
		t.Fatalf("args length: got %d want %d: %#v", len(plan.Args), len(want), plan.Args)
	}
	for i := range want {
		if plan.Args[i] != want[i] {
			t.Fatalf("arg %d: got %#v want %#v; all=%#v", i, plan.Args[i], want[i], plan.Args)
		}
	}
}

func TestBuildRecommendBidPlanUsesWeightedAverage(t *testing.T) {
	plan, err := buildRecommendBidPlan(TrafficSegmentRequest{
		FormatType:  "push",
		TrafficType: "adult",
	}, "traffic_volume_hourly")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, plan.SQL, "sum(nonzero_win_dsp_price_sum)")
	mustContain(t, plan.SQL, "/ sum(nonzero_win_dsp_price_count)")
	mustContain(t, plan.SQL, "AS average_bid")
	if len(plan.Args) != 2 || plan.Args[0] != "IPP" || plan.Args[1] != "ADULT" {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
}

func TestBuildTrafficPlanEmptyListsIgnoreModes(t *testing.T) {
	plan, err := buildCalculatorPlan(TrafficSegmentRequest{
		FormatType:  "native",
		TrafficType: "mainstream",
		CountryMode: FilterMode("bad-but-unused"),
	}, "traffic_volume_hourly")
	if err != nil {
		t.Fatalf("empty list mode must be ignored: %v", err)
	}
	mustNotContain(t, plan.SQL, "geo IN")
	mustNotContain(t, plan.SQL, "geo NOT IN")
}

func TestBuildTrafficPlanRejectsInvalidMode(t *testing.T) {
	_, err := buildCalculatorPlan(TrafficSegmentRequest{
		FormatType:  "banner",
		TrafficType: "adult",
		Country:     []string{"DE"},
		CountryMode: FilterMode("unknown"),
	}, "traffic_volume_hourly")
	if err == nil || !strings.Contains(err.Error(), "invalid country_mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTrafficPlanRejectsMissingRequiredEnums(t *testing.T) {
	_, err := buildCalculatorPlan(TrafficSegmentRequest{
		TrafficType: "adult",
	}, "traffic_volume_hourly")
	if err == nil || !strings.Contains(err.Error(), "invalid format_type") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = buildCalculatorPlan(TrafficSegmentRequest{
		FormatType: "banner",
	}, "traffic_volume_hourly")
	if err == nil || !strings.Contains(err.Error(), "invalid traffic_type") {
		t.Fatalf("unexpected error: %v", err)
	}
}
