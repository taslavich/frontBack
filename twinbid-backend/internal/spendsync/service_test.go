package spendsync

import (
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
