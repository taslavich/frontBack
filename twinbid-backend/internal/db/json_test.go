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
