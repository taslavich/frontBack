package percenterbilling

import "testing"

func TestValidateApplyRequest(t *testing.T) {
	valid := ApplyRequest{EventID: "event-1", UserID: "user-1", CampaignID: "campaign-1", PromoGeneration: 4, SpendDelta: 0.25}
	if err := validateApplyRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	cases := []ApplyRequest{
		{UserID: "user-1", CampaignID: "campaign-1", SpendDelta: 0.25},
		{EventID: "event-1", CampaignID: "campaign-1", SpendDelta: 0.25},
		{EventID: "event-1", UserID: "user-1", SpendDelta: 0.25},
		{EventID: "event-1", UserID: "user-1", CampaignID: "campaign-1", PromoGeneration: -1, SpendDelta: 0.25},
		{EventID: "event-1", UserID: "user-1", CampaignID: "campaign-1", PromoGeneration: 4, SpendDelta: 0},
	}
	for i, req := range cases {
		if err := validateApplyRequest(req); err == nil {
			t.Fatalf("case %d: invalid request accepted: %+v", i, req)
		}
	}
}

func TestParsePromoStateIncludesGeneration(t *testing.T) {
	state, err := parsePromoState("12.5", 7, 4)
	if err != nil {
		t.Fatal(err)
	}
	if state.Remaining != 12.5 || state.Revision != 7 || state.Generation != 4 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if _, err := parsePromoState("1", 1, -1); err == nil {
		t.Fatal("negative promo generation accepted")
	}
}
