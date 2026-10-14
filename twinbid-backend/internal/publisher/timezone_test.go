package publisher

import (
	"testing"
	"time"
)

func TestLegacyTimezone(t *testing.T) {
	for raw, want := range map[string]string{"": "UTC", "utc_0": "UTC", "utc_3": "Etc/GMT-3", "utc_-5": "Etc/GMT+5", "Europe/Moscow": "Europe/Moscow", "UTC": "UTC"} {
		got, e := NormalizeTimezone(raw)
		if e != nil || got != want {
			t.Fatal(raw, got, e)
		}
	}
	got, _ := NormalizeTimezone("utc_3")
	loc, _ := time.LoadLocation(got)
	v := time.Date(2026, 10, 13, 22, 30, 0, 0, time.UTC).In(loc)
	if v.Day() != 14 || v.Hour() != 1 {
		t.Fatal("hour/day grouping changed", v)
	}
	for _, raw := range []string{"utc_x", "utc_25", "Mars/Unknown"} {
		if _, e := NormalizeTimezone(raw); e == nil {
			t.Fatal(raw)
		}
	}
}
