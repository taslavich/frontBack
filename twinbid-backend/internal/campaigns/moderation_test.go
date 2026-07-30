package campaigns

import (
	"errors"
	"net/http"
	"testing"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

func TestApplyModerationDecision(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		decision   string
		wantStatus string
	}{
		{name: "approve", status: "moderation", decision: "approve", wantStatus: "waiting"},
		{name: "reject", status: "moderation", decision: "reject", wantStatus: "draft"},
		{name: "normalizes decision", status: "moderation", decision: " APPROVE ", wantStatus: "waiting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			campaign := models.Campaign{Status: tt.status}
			if err := applyModerationDecision(&campaign, tt.decision); err != nil {
				t.Fatalf("applyModerationDecision() error = %v", err)
			}
			if campaign.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", campaign.Status, tt.wantStatus)
			}
		})
	}
}

func TestApplyModerationDecisionRejectsCancelledCampaign(t *testing.T) {
	campaign := models.Campaign{Status: "draft"}
	err := applyModerationDecision(&campaign, "approve")

	var httpErr httpx.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want httpx.HTTPError", err)
	}
	if httpErr.Status != http.StatusConflict {
		t.Fatalf("status = %d, want %d", httpErr.Status, http.StatusConflict)
	}
	if httpErr.Message != "Модерация уже отменена пользователем" {
		t.Fatalf("message = %q", httpErr.Message)
	}
	if campaign.Status != "draft" {
		t.Fatalf("campaign mutated on conflict: status=%q", campaign.Status)
	}
}

func TestApplyModerationDecisionRejectsCompletedModeration(t *testing.T) {
	campaign := models.Campaign{Status: "waiting"}
	err := applyModerationDecision(&campaign, "reject")

	var httpErr httpx.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict {
		t.Fatalf("error = %#v, want conflict", err)
	}
	if campaign.Status != "waiting" {
		t.Fatalf("campaign mutated on conflict: status=%q", campaign.Status)
	}
}

func TestApplyModerationDecisionRejectsUnknownDecision(t *testing.T) {
	campaign := models.Campaign{Status: "moderation"}
	err := applyModerationDecision(&campaign, "activate")

	var httpErr httpx.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("error = %#v, want bad request", err)
	}
	if campaign.Status != "moderation" {
		t.Fatalf("campaign mutated on validation error: status=%q", campaign.Status)
	}
}

func TestHandlerValidBotSecret(t *testing.T) {
	handler := &Handler{botSecret: "shared-secret"}
	if !handler.validBotSecret("shared-secret") {
		t.Fatal("expected matching secret to be accepted")
	}
	if handler.validBotSecret("wrong-secret") {
		t.Fatal("expected wrong secret to be rejected")
	}
	if (&Handler{}).validBotSecret("") {
		t.Fatal("empty configured secret must not authorize requests")
	}
}
