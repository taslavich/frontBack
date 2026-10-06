package spendsync

import (
	"math/big"
	"strings"
	"testing"

	"twinbid-backend/internal/stats"
)

const (
	userID     = "11111111-1111-4111-8111-111111111111"
	campaignID = "22222222-2222-4222-8222-222222222222"
)

func TestSplitTotals(t *testing.T) {
	userIDs, userAmounts, campaignIDs, campaignAmounts, skippedInvalidEntityRows, invalidEntityIDSamples, err := splitTotals([]stats.CumulativeSpendTotal{
		{EntityType: "campaign", EntityID: campaignID, Amount: "4.9995"},
		{EntityType: "user", EntityID: userID, Amount: "12.345678901234"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skippedInvalidEntityRows != 0 || len(invalidEntityIDSamples) != 0 {
		t.Fatalf("unexpected invalid entity IDs: skipped=%d samples=%v", skippedInvalidEntityRows, invalidEntityIDSamples)
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
		{EntityType: "user", EntityID: userID, Amount: "NaN"},
		{EntityType: "campaign", EntityID: campaignID, Amount: "-1"},
	}
	for _, total := range tests {
		if _, _, _, _, _, _, err := splitTotals([]stats.CumulativeSpendTotal{total}); err == nil {
			t.Fatalf("expected error for %#v", total)
		}
	}
}

func TestSplitTotalsSkipsInvalidEntityIDWithoutBlockingValidTotals(t *testing.T) {
	const poisonCampaignID = "AlNDDRoGHQwYWwgCJSptJywnbHBuejF_"

	userIDs, userAmounts, campaignIDs, campaignAmounts, skippedInvalidEntityRows, invalidEntityIDSamples, err := splitTotals([]stats.CumulativeSpendTotal{
		{EntityType: "campaign", EntityID: poisonCampaignID, Amount: "0.0004216000060551"},
		{EntityType: "user", EntityID: userID, Amount: "12.5"},
		{EntityType: "campaign", EntityID: campaignID, Amount: "4.5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skippedInvalidEntityRows != 1 {
		t.Fatalf("expected one skipped invalid entity row, got %d", skippedInvalidEntityRows)
	}
	if len(invalidEntityIDSamples) != 1 || !strings.Contains(invalidEntityIDSamples[0], poisonCampaignID) {
		t.Fatalf("unexpected invalid entity ID samples: %v", invalidEntityIDSamples)
	}
	if len(userIDs) != 1 || userIDs[0] != userID || userAmounts[0] != "12.5" {
		t.Fatalf("valid user total was not preserved: ids=%v amounts=%v", userIDs, userAmounts)
	}
	if len(campaignIDs) != 1 || campaignIDs[0] != campaignID || campaignAmounts[0] != "4.5" {
		t.Fatalf("valid campaign total was not preserved: ids=%v amounts=%v", campaignIDs, campaignAmounts)
	}
}

func TestUserSpendSyncConsumesPromoFromNewClickHouseSpendOnly(t *testing.T) {
	query := strings.ToLower(updateUsersCumulativeSpendSQL)
	for _, want := range []string{
		"promo_spend_remaining",
		"incoming.cum_done_dollars - u.promo_spend_synced",
		"greatest(incoming.cum_done_dollars - u.promo_spend_synced, 0)",
		"promo_spend_synced = greatest",
		"cum_done_dollars = incoming.cum_done_dollars",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("ClickHouse promo spend sync query missing %q: %s", want, updateUsersCumulativeSpendSQL)
		}
	}
	if strings.Contains(query, "promo_generation") {
		t.Fatalf("ClickHouse spend sync must not depend on promo generations: %s", updateUsersCumulativeSpendSQL)
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

func TestNormalizeRecoveryTotalsSortsAndPreservesDecimalText(t *testing.T) {
	totals, err := normalizeRecoveryTotals([]stats.POPRecoveredSpendTotal{
		{UserID: userID, CampaignID: "33333333-3333-4333-8333-333333333333", Amount: "2.500000000001"},
		{UserID: userID, CampaignID: campaignID, Amount: "0"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(totals) != 2 {
		t.Fatalf("unexpected total count: %d", len(totals))
	}
	if totals[0].CampaignID != campaignID || totals[0].Amount != "0" {
		t.Fatalf("unexpected first recovery total: %#v", totals[0])
	}
	if totals[1].Amount != "2.500000000001" {
		t.Fatalf("decimal text changed unexpectedly: %#v", totals[1])
	}
}

func TestNormalizeRecoveryTotalsRejectsDuplicateCampaignAndInvalidAmount(t *testing.T) {
	if _, err := normalizeRecoveryTotals([]stats.POPRecoveredSpendTotal{
		{UserID: userID, CampaignID: campaignID, Amount: "1"},
		{UserID: userID, CampaignID: campaignID, Amount: "2"},
	}); err == nil {
		t.Fatal("expected duplicate campaign to be rejected")
	}
	if _, err := normalizeRecoveryTotals([]stats.POPRecoveredSpendTotal{
		{UserID: userID, CampaignID: campaignID, Amount: "-0.01"},
	}); err == nil {
		t.Fatal("expected negative recovery total to be rejected")
	}
}

func TestDecimalStringKeepsTwelveDecimalRecoveryPrecision(t *testing.T) {
	value, ok := new(big.Rat).SetString("2.500000000001")
	if !ok {
		t.Fatal("cannot parse test decimal")
	}
	if got := decimalString(value); got != "2.500000000001" {
		t.Fatalf("unexpected decimal: %q", got)
	}
}
