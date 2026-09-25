package db

import "testing"

func TestUnmarshalMacroMapSupportsLegacyAndStringValues(t *testing.T) {
	got, err := UnmarshalMacroMap([]byte(`{
		"site_id": true,
		"campaign_id": 1,
		"creative_id": false,
		"country_code": 0,
		"click_id": "subid",
		"browser": "browser_name"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"site_id":     "site_id",
		"campaign_id": "campaign_id",
		"click_id":    "subid",
		"browser":     "browser_name",
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("macro %q got %q want %q", key, got[key], value)
		}
	}
}

func TestUnmarshalVideoCreativeMetadata(t *testing.T) {
	got, err := UnmarshalVideoCreativeMetadata([]byte(`{
		"mimes":[" Video/MP4 ","video/mp4"],
		"duration":20,
		"protocols":[2,3,2],
		"bitrate":1200,
		"linearity":1
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Mimes) != 1 || got.Mimes[0] != "video/mp4" {
		t.Fatalf("unexpected mimes: %#v", got)
	}
	if len(got.Protocols) != 2 || got.Duration != 20 || got.Bitrate != 1200 || got.Linearity != 1 {
		t.Fatalf("unexpected video metadata: %#v", got)
	}
}
