package bot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCampaignModerationRequestRTBContract(t *testing.T) {
	link := "https://buyer.example/openrtb"
	payload, err := json.Marshal(CampaignModerationRequest{
		CampaignID: "campaign-1",
		RTB:        true,
		DSPLink:    &link,
		Creatives:  []CreativePayload{},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(payload)
	for _, want := range []string{
		`"rtb":true`,
		`"dsp_link":"https://buyer.example/openrtb"`,
		`"creatives":[]`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("payload %s does not contain %s", body, want)
		}
	}
}
