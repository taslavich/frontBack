package poprecovery

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestAdjustmentValidate(t *testing.T) {
	valid := Adjustment{
		ID:         "11111111-1111-4111-8111-111111111111",
		UserID:     "22222222-2222-4222-8222-222222222222",
		CampaignID: "33333333-3333-4333-8333-333333333333",
		Delta:      "1.25",
		DeltaFloat: 1.25,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid adjustment rejected: %v", err)
	}

	invalid := valid
	invalid.Delta = "-1"
	invalid.DeltaFloat = -1
	if err := invalid.Validate(); err == nil {
		t.Fatal("negative delta must be rejected")
	}
}

func TestFormatHealthLogContainsBootstrapCursorPendingAndLastError(t *testing.T) {
	health := Health{
		BootstrapCompleted: true,
		CursorRows:         3,
		CursorTotal:        "12.5",
		PendingAdjustments: 2,
		LastError:          "redis unavailable",
		LastErrorAt:        sql.NullTime{Time: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	got := FormatHealthLog(health)
	for _, want := range []string{
		"bootstrap_completed=true",
		"cursor_rows=3",
		"cursor_total=12.5",
		"pending_adjustments=2",
		`last_error="redis unavailable"`,
		"2026-10-06T00:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("health log missing %q: %s", want, got)
		}
	}
}
