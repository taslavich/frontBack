package percenterbilling

import "testing"

func TestValidateApplyRequest(t *testing.T) {
	valid := ApplyRequest{EventID: "event-1", UserID: "user-1", CampaignID: "campaign-1", SpendDelta: 0.25}
	if err := validateApplyRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	cases := []ApplyRequest{
		{UserID: "user-1", CampaignID: "campaign-1", SpendDelta: 0.25},
		{EventID: "event-1", CampaignID: "campaign-1", SpendDelta: 0.25},
		{EventID: "event-1", UserID: "user-1", SpendDelta: 0.25},
		{EventID: "event-1", UserID: "user-1", CampaignID: "campaign-1", SpendDelta: 0},
	}
	for i, req := range cases {
		if err := validateApplyRequest(req); err == nil {
			t.Fatalf("case %d: invalid request accepted: %+v", i, req)
		}
	}
}
