package stats

import (
	"strings"
	"testing"
)

func TestBuildOSVersionPredicateBranchBoundaries(t *testing.T) {
	predicate, args, err := buildOSVersionPredicate([]string{"iOS 18.7", "Android 12L"})
	if err != nil {
		t.Fatalf("buildOSVersionPredicate: %v", err)
	}
	for _, want := range []string{"lowerUTF8(os) IN ('ios'", "toUInt64OrNull", "lowerUTF8(os) IN ('android'"} {
		if !strings.Contains(predicate, want) {
			t.Fatalf("predicate %q missing %q", predicate, want)
		}
	}
	wantArgs := []any{uint64(18), uint64(7), "12l"}
	if len(args) != len(wantArgs) {
		t.Fatalf("args=%v want=%v", args, wantArgs)
	}
	for i := range args {
		if args[i] != wantArgs[i] {
			t.Fatalf("args[%d]=%v want=%v", i, args[i], wantArgs[i])
		}
	}
}

func TestBuildOSVersionPredicateRejectsPatchTarget(t *testing.T) {
	if _, _, err := buildOSVersionPredicate([]string{"iOS 18.7.1"}); err == nil {
		t.Fatal("expected patch-level campaign/stat filter to be rejected")
	}
}

func TestCalculatorOSVersionIncludeExclude(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode FilterMode
		want string
	}{
		{name: "include", mode: FilterModeInclude, want: "AND ((lowerUTF8(os)"},
		{name: "exclude", mode: FilterModeExclude, want: "AND NOT ((lowerUTF8(os)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := buildCalculatorPlan(TrafficSegmentRequest{
				FormatType: "banner", TrafficType: "mainstream",
				OSVersion: []string{"Android 8.1"}, OSVersionMode: tc.mode,
			}, "ads.traffic_volume_hourly")
			if err != nil {
				t.Fatalf("buildCalculatorPlan: %v", err)
			}
			if !strings.Contains(plan.SQL, tc.want) {
				t.Fatalf("SQL missing %q:\n%s", tc.want, plan.SQL)
			}
			if len(plan.Args) < 4 || plan.Args[len(plan.Args)-2] != uint64(8) || plan.Args[len(plan.Args)-1] != uint64(1) {
				t.Fatalf("unexpected args: %#v", plan.Args)
			}
		})
	}
}

func TestStatsGroupAndFilterOSVersion(t *testing.T) {
	rows, _, err := buildStatsQueries(testUserID, QueryRequest{
		From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByOSVersion,
		Filters: map[string][]string{"os_version": {"iOS 18"}},
	}, "ads.agg_stats")
	if err != nil {
		t.Fatalf("buildStatsQueries: %v", err)
	}
	if !strings.Contains(rows.SQL, "'iOS'") || !strings.Contains(rows.SQL, "os, os_version") {
		t.Fatalf("OS-version grouping missing:\n%s", rows.SQL)
	}
	if rows.Args[len(rows.Args)-1] != uint64(18) {
		t.Fatalf("unexpected args: %#v", rows.Args)
	}
}
