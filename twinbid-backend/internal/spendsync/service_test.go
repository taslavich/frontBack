package spendsync

import (
	"strings"
	"testing"

	"twinbid-backend/internal/stats"
)

const (
	userID     = "11111111-1111-4111-8111-111111111111"
	campaignID = "22222222-2222-4222-8222-222222222222"
)

func TestSplitTotals(t *testing.T) {
	userIDs, userAmounts, campaignIDs, campaignAmounts, err := splitTotals([]stats.CumulativeSpendTotal{
		{EntityType: "campaign", EntityID: campaignID, Amount: "4.9995"},
		{EntityType: "user", EntityID: userID, Amount: "12.345678901234"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userIDs) != 1 || userIDs[0] != userID || userAmounts[0] != "12.345678901234" {
		t.Fatalf("unexpected user totals: ids=%v amounts=%v", userIDs, userAmounts)
	}
	if len(campaignIDs) != 1 || campaignIDs[0] != campaignID || campaignAmounts[0] != "4.9995" {
		t.Fatalf("unexpected campaign totals: ids=%v amounts=%v", campaignIDs, campaignAmounts)
	}
}

func TestSplitTotalsRejectsInvalidRows(t *testing.T) {
	tests := []stats.CumulativeSpendTotal{
		{EntityType: "other", EntityID: userID, Amount: "1"},
		{EntityType: "user", EntityID: "not-a-uuid", Amount: "1"},
		{EntityType: "user", EntityID: userID, Amount: "NaN"},
		{EntityType: "campaign", EntityID: campaignID, Amount: "-1"},
	}
	for _, total := range tests {
		if _, _, _, _, err := splitTotals([]stats.CumulativeSpendTotal{total}); err == nil {
			t.Fatalf("expected error for %#v", total)
		}
	}
}

func TestUserSpendSyncDoesNotMutatePromoState(t *testing.T) {
	query := strings.ToLower(updateUsersCumulativeSpendSQL)
	if strings.Contains(query, "promo_spend_remaining") || strings.Contains(query, "promo_revision") {
		t.Fatalf("minute spend sync must not mutate realtime promo state: %s", updateUsersCumulativeSpendSQL)
	}
	if !strings.Contains(query, "cum_done_dollars") {
		t.Fatalf("spend sync query no longer updates cumulative spend: %s", updateUsersCumulativeSpendSQL)
	}
}

func TestNoBudgetTransitionRunsInSpendSync(t *testing.T) {
	query := strings.ToLower(markNoBudgetCampaignsSQL)
	for _, want := range []string{
		"update campaigns",
		"status = 'no_budget'",
		"status = 'active'",
		"goal_total_dollars - c.cum_done_dollars",
		"base_price / 1000",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("no-budget transition query missing %q: %s", want, markNoBudgetCampaignsSQL)
		}
	}
	if strings.Contains(query, "no_budget_notified = true") {
		t.Fatalf("critical spend-sync path must not wait for notification delivery: %s", markNoBudgetCampaignsSQL)
	}
}

func TestCampaignSpendUpdateAndNoBudgetTransitionAreSeparateStatements(t *testing.T) {
	update := strings.ToLower(updateCampaignsCumulativeSpendSQL)
	if !strings.Contains(update, "set cum_done_dollars") {
		t.Fatalf("campaign spend update no longer writes cum_done_dollars: %s", updateCampaignsCumulativeSpendSQL)
	}
	if strings.Contains(update, "status = 'no_budget'") {
		t.Fatalf("campaign status should be evaluated after all cumulative spend rows are updated: %s", updateCampaignsCumulativeSpendSQL)
	}
}
