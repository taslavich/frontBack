package pubcontract

import (
	"strings"
	"testing"
)

func TestNormalizeAndIdentity(t *testing.T) {
	for _, v := range []string{"https://WWW.Example.COM./path", "http://example.com:80/p", "https://example.com"} {
		host, _, e := Page(v)
		if e != nil || host != "example.com" {
			t.Fatalf("%s -> %s: %v", v, host, e)
		}
	}
	for _, v := range []string{"https://example.test", "https://test.example.com", "https://пример.рф"} {
		if _, _, e := Page(v); e != nil {
			t.Fatal(v, e)
		}
	}
	for _, v := range []string{"", "null", "https://localhost", "http://a.localhost", "http://127.0.0.1", "https://10.0.0.1", "http://[::1]", "about:blank", "https://name", "https://www. bad.com", "https://user:pass@example.com"} {
		if _, _, e := Page(v); e == nil {
			t.Fatalf("accepted %q", v)
		}
	}
	if SiteID("example.com") == SiteID("news.example.com") {
		t.Fatal("subdomains collapsed")
	}
	if ZoneID(SiteID("example.com"), "code-a") == ZoneID(SiteID("example.com"), "code-b") {
		t.Fatal("codes collapsed")
	}
	if Hash("zone", "1", "23") == Hash("zone", "12", "3") {
		t.Fatal("ambiguous encoding")
	}
	if got := SiteID("example.com"); got != "a86288ef019d27ad81f8b08700e49312b009377421841267df68903d2d3329d0" {
		t.Fatal(got)
	}
	if got := ZoneID(SiteID("example.com"), "code-a"); got != "cff5d4fc9106578ea1aa4f64163cf8341544f70fc0313c3af618a174933c0ed6" {
		t.Fatal(got)
	}
}
func TestHMAC(t *testing.T) {
	key := strings.Repeat("x", 32)
	a := Attribution{UserID: "owner", RequestID: "r"}
	sign := Sign(key, a)
	if !Verify(key, a, sign) {
		t.Fatal("signature rejected")
	}
	a.UserID = "other"
	if Verify(key, a, sign) {
		t.Fatal("changed owner accepted")
	}
	if Verify("short", a, sign) {
		t.Fatal("weak key accepted")
	}
}
func TestConfigValidation(t *testing.T) {
	v := Code{Name: "a", Format: "popunder", TrafficType: "mainstream", Device: "all", TriggerClick: 1, FrequencyMinutes: .1}
	if e := v.Validate(); e != nil {
		t.Fatal(e)
	}
	v.FrequencyMinutes = .09
	if v.Validate() == nil {
		t.Fatal("interval <6 seconds")
	}
}
func TestClusterGuardSlots(t *testing.T) {
	if redisSlot("123456789") != 12739 {
		t.Fatal("wrong Redis CRC16")
	}
	for slot := 0; slot < 16384; slot++ {
		key := GuardKeyForSlot(slot)
		tag := strings.Split(strings.Split(key, "{")[1], "}")[0]
		if redisSlot(tag) != slot {
			t.Fatal(slot, key)
		}
	}
}
