package spendsync

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"twinbid-backend/internal/stats"
)

const (
	userID      = "11111111-1111-4111-8111-111111111111"
	campaignID  = "22222222-2222-4222-8222-222222222222"
	campaignID2 = "33333333-3333-4333-8333-333333333333"
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
		{EntityType: "campaign", EntityID: campaignID, Amount: "4.5"},
		{EntityType: "campaign", EntityID: poisonCampaignID, Amount: "0.0004216000060551"},
		{EntityType: "campaign", EntityID: campaignID2, Amount: "6.5"},
		{EntityType: "user", EntityID: userID, Amount: "12.5"},
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
	if len(campaignIDs) != 2 || campaignIDs[0] != campaignID || campaignAmounts[0] != "4.5" || campaignIDs[1] != campaignID2 || campaignAmounts[1] != "6.5" {
		t.Fatalf("valid campaign totals around invalid row were not preserved: ids=%v amounts=%v", campaignIDs, campaignAmounts)
	}
}

func TestSplitTotalsRequiresCanonicalInternalUUID(t *testing.T) {
	for _, nonCanonical := range []string{
		"22222222-2222-4222-8222-22222222222A",
		"22222222222242228222222222222222",
		"{22222222-2222-4222-8222-222222222222}",
	} {
		_, _, campaignIDs, _, skipped, samples, err := splitTotals([]stats.CumulativeSpendTotal{
			{EntityType: "campaign", EntityID: nonCanonical, Amount: "1"},
		})
		if err != nil {
			t.Fatalf("non-canonical entity must be skipped, not fatal: id=%q err=%v", nonCanonical, err)
		}
		if len(campaignIDs) != 0 || skipped != 1 || len(samples) != 1 {
			t.Fatalf("non-canonical entity was not skipped: id=%q campaigns=%v skipped=%d samples=%v", nonCanonical, campaignIDs, skipped, samples)
		}
	}
}

type recoveryReachabilitySource struct {
	recoveryCalled bool
}

func (s *recoveryReachabilitySource) CumulativeSpend(context.Context) ([]stats.CumulativeSpendTotal, error) {
	return []stats.CumulativeSpendTotal{
		{EntityType: "campaign", EntityID: "AlNDDRoGHQwYWwgCJSptJywnbHBuejF_", Amount: "1"},
	}, nil
}

func (s *recoveryReachabilitySource) POPRecoveryEventsAfter(context.Context, stats.POPRecoveryCursor, int) ([]stats.POPRecoveryEvent, error) {
	s.recoveryCalled = true
	return nil, nil
}

func TestRecoveryReachabilitySourceImplementsEventSource(t *testing.T) {
	var _ source = (*recoveryReachabilitySource)(nil)
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

func TestNormalizeRecoveryTotalsSkipsInvalidNonInternalIDs(t *testing.T) {
	totals, err := normalizeRecoveryTotals([]stats.POPRecoveredSpendTotal{
		{UserID: userID, CampaignID: campaignID, Amount: "1"},
		{UserID: userID, CampaignID: "AlNDDRoGHQwYWwgCJSptJywnbHBuejF_", Amount: "2"},
		{UserID: "not-an-internal-user", CampaignID: campaignID2, Amount: "3"},
	})
	if err != nil {
		t.Fatalf("invalid/non-internal IDs must be skipped, not abort POP recovery normalization: %v", err)
	}
	if len(totals) != 1 || totals[0].CampaignID != campaignID || totals[0].UserID != userID {
		t.Fatalf("unexpected normalized recovery totals after invalid rows were skipped: %#v", totals)
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

func TestSplitTotalsRecordsDiagnosticOnlyFilteredEntityWithoutAffectingValidTotals(t *testing.T) {
	const poisonCampaignID = "AlNDDRoGHQwYWwgCJSptJywnbHBuejF_"

	userIDs, userAmounts, campaignIDs, campaignAmounts, skipped, samples, err := splitTotals([]stats.CumulativeSpendTotal{
		{EntityType: "campaign", EntityID: campaignID, Amount: "4.5"},
		{EntityType: "campaign", EntityID: poisonCampaignID, Amount: "0", DiagnosticOnly: true},
		{EntityType: "user", EntityID: userID, Amount: "12.5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skipped != 1 || len(samples) != 1 || !strings.Contains(samples[0], poisonCampaignID) {
		t.Fatalf("diagnostic filtered entity was not surfaced correctly: skipped=%d samples=%v", skipped, samples)
	}
	if len(userIDs) != 1 || userIDs[0] != userID || userAmounts[0] != "12.5" {
		t.Fatalf("valid user total changed: ids=%v amounts=%v", userIDs, userAmounts)
	}
	if len(campaignIDs) != 1 || campaignIDs[0] != campaignID || campaignAmounts[0] != "4.5" {
		t.Fatalf("valid campaign total changed: ids=%v amounts=%v", campaignIDs, campaignAmounts)
	}
}

func TestNormalizeRecoveryEventsGroupsOnlyNewSlice(t *testing.T) {
	start := stats.POPRecoveryCursor{RecoveryAtMS: 1000, SourceKey: "a"}
	events := []stats.POPRecoveryEvent{
		{RecoveryAtMS: 1001, SourceKey: "b", UserID: userID, CampaignID: campaignID, Amount: "1.25"},
		{RecoveryAtMS: 1001, SourceKey: "c", UserID: userID, CampaignID: campaignID, Amount: "0.75"},
		{RecoveryAtMS: 1002, SourceKey: "a", UserID: userID, CampaignID: campaignID2, Amount: "3"},
	}
	groups, end, err := normalizeRecoveryEvents(events, start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end.RecoveryAtMS != 1002 || end.SourceKey != "a" {
		t.Fatalf("unexpected end cursor: %#v", end)
	}
	if len(groups) != 2 {
		t.Fatalf("unexpected group count: %d", len(groups))
	}
	if groups[0].CampaignID != campaignID || groups[0].Delta != "2" {
		t.Fatalf("unexpected first group: %#v", groups[0])
	}
	if groups[1].CampaignID != campaignID2 || groups[1].Delta != "3" {
		t.Fatalf("unexpected second group: %#v", groups[1])
	}
}

func TestNormalizeRecoveryEventsRejectsNonMonotonicMarker(t *testing.T) {
	_, _, err := normalizeRecoveryEvents([]stats.POPRecoveryEvent{{
		RecoveryAtMS: 1000, SourceKey: "a", UserID: userID, CampaignID: campaignID, Amount: "1",
	}}, stats.POPRecoveryCursor{RecoveryAtMS: 1000, SourceKey: "a"})
	if err == nil {
		t.Fatal("expected duplicate/non-advancing event marker to be rejected")
	}
}
