package campaigns

import (
	"encoding/json"
	"testing"
)

func TestPatchCampaignRequestBlockVPNPresence(t *testing.T) {
	var omitted PatchCampaignRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("unmarshal omitted block_vpn: %v", err)
	}
	if omitted.BlockVPN != nil {
		t.Fatalf("omitted block_vpn must stay nil, got %v", *omitted.BlockVPN)
	}

	var disabled PatchCampaignRequest
	if err := json.Unmarshal([]byte(`{"block_vpn":false}`), &disabled); err != nil {
		t.Fatalf("unmarshal block_vpn=false: %v", err)
	}
	if disabled.BlockVPN == nil || *disabled.BlockVPN {
		t.Fatalf("explicit block_vpn=false must be preserved")
	}

	var enabled PatchCampaignRequest
	if err := json.Unmarshal([]byte(`{"block_vpn":true}`), &enabled); err != nil {
		t.Fatalf("unmarshal block_vpn=true: %v", err)
	}
	if enabled.BlockVPN == nil || !*enabled.BlockVPN {
		t.Fatalf("explicit block_vpn=true must be preserved")
	}
}
