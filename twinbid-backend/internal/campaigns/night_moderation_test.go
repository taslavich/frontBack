package campaigns

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

func TestNightModerationWindowStartUTC(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		active    bool
		wantStart time.Time
	}{
		{name: "before window", now: time.Date(2026, 9, 29, 17, 59, 59, 0, time.UTC), active: false},
		{name: "start boundary", now: time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC), active: true, wantStart: time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)},
		{name: "late evening", now: time.Date(2026, 9, 29, 23, 59, 0, 0, time.UTC), active: true, wantStart: time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)},
		{name: "after midnight", now: time.Date(2026, 9, 30, 0, 1, 0, 0, time.UTC), active: true, wantStart: time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)},
		{name: "before end", now: time.Date(2026, 9, 30, 5, 59, 59, 0, time.UTC), active: true, wantStart: time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)},
		{name: "end boundary", now: time.Date(2026, 9, 30, 6, 0, 0, 0, time.UTC), active: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, active := NightModerationWindowStart(tt.now)
			if active != tt.active {
				t.Fatalf("active=%v want=%v", active, tt.active)
			}
			if tt.active && !got.Equal(tt.wantStart) {
				t.Fatalf("start=%s want=%s", got, tt.wantStart)
			}
		})
	}
}

func TestNextNightModerationWindowStart(t *testing.T) {
	if got := NextNightModerationWindowStart(time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)); !got.Equal(time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("next before 18:00 = %s", got)
	}
	if got := NextNightModerationWindowStart(time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)); !got.Equal(time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("next at 18:00 = %s", got)
	}
}

func TestApplyNightModerationDecisionApprovePreservesCurrentStatus(t *testing.T) {
	campaign := models.Campaign{Status: "active"}
	state := &NightModerationState{}
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	if err := applyNightModerationDecision(&campaign, state, "approve", now); err != nil {
		t.Fatalf("applyNightModerationDecision() error = %v", err)
	}
	if campaign.Status != "active" {
		t.Fatalf("status=%q want active", campaign.Status)
	}
	if state.ManualDecision != "approve" || state.ManualDecidedAt == nil || !state.ManualDecidedAt.Equal(now) {
		t.Fatalf("state=%+v", state)
	}
}

func TestApplyNightModerationDecisionRejectMovesToDraft(t *testing.T) {
	campaign := models.Campaign{Status: "active"}
	state := &NightModerationState{}
	if err := applyNightModerationDecision(&campaign, state, "reject", time.Now()); err != nil {
		t.Fatalf("applyNightModerationDecision() error = %v", err)
	}
	if campaign.Status != "draft" {
		t.Fatalf("status=%q want draft", campaign.Status)
	}
	if state.ManualDecision != "reject" {
		t.Fatalf("decision=%q", state.ManualDecision)
	}
}

func TestApplyNightModerationDecisionIsIdempotentForSameDecision(t *testing.T) {
	campaign := models.Campaign{Status: "waiting"}
	state := &NightModerationState{ManualDecision: "approve"}
	if err := applyNightModerationDecision(&campaign, state, "approve", time.Now()); err != nil {
		t.Fatalf("same decision must be idempotent: %v", err)
	}
	if campaign.Status != "waiting" {
		t.Fatalf("status=%q", campaign.Status)
	}
}

func TestApplyNightModerationDecisionRejectsSecondDifferentDecision(t *testing.T) {
	campaign := models.Campaign{Status: "active"}
	state := &NightModerationState{ManualDecision: "approve"}
	err := applyNightModerationDecision(&campaign, state, "reject", time.Now())
	var httpErr httpx.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict {
		t.Fatalf("error=%#v want conflict", err)
	}
	if campaign.Status != "active" {
		t.Fatalf("campaign mutated after final decision: %q", campaign.Status)
	}
}
