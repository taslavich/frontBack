package publisherconfig

import "testing"

func TestDirectLinkOrigin(t *testing.T) {
	cfg := Config{PublicURL: "https://pub.example.test/"}
	if err := cfg.validateDirectLinkOrigin(); err != nil || cfg.DirectLinkOrigin() != "https://pub.example.test" {
		t.Fatalf("fallback: %q, %v", cfg.DirectLinkOrigin(), err)
	}
	cfg.DirectLinkPublicURL = "https://carloverrcvbg.site/"
	if err := cfg.validateDirectLinkOrigin(); err != nil || cfg.DirectLinkOrigin() != "https://carloverrcvbg.site" {
		t.Fatalf("override: %q, %v", cfg.DirectLinkOrigin(), err)
	}
	for _, invalid := range []string{"http://example.test", "https://example.test/go", "https://example.test?", "https://example.test#", "https://user@example.test"} {
		cfg.DirectLinkPublicURL = invalid
		if cfg.validateDirectLinkOrigin() == nil {
			t.Errorf("accepted invalid origin %q", invalid)
		}
	}
}
