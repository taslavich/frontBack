package pubcontract

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestShortIDsIndependentGoldenCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/short_ids_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Host, Code, Site, Zone string }
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.Host+"/"+row.Code, func(t *testing.T) {
			site := SiteID(row.Host)
			zone := ZoneID(site, row.Code)
			if site != row.Site || zone != row.Zone || !ValidShortID(site) || !ValidShortID(zone) {
				t.Fatal(site, zone, row)
			}
			if BlockKey(site) != Prefix+"block:"+site {
				t.Fatal("ready Site ID was hashed again")
			}
		})
	}
	// Other full hashes and unambiguous input framing retain their old contract.
	if Hash("site", "example.com") != "a86288ef019d27ad81f8b08700e49312b009377421841267df68903d2d3329d0" {
		t.Fatal("full hash changed")
	}
	if ZoneID("1", "23") == ZoneID("12", "3") {
		t.Fatal("ambiguous framing")
	}
}

func TestShortIDFormatAndNormalization(t *testing.T) {
	for _, id := range []string{"000000000000", "0123456789ab", "aAzZ01234567"} {
		if !ValidShortID(id) {
			t.Fatal("valid ID rejected", id)
		}
	}
	for _, id := range []string{"", "12345678901", "1234567890123", strings.Repeat("a", 64), "example.com", "ABCDEFGHIJK_", "ABCDEFGHIJK-", "ABCDEFGHIJK ", "ABCDEFGHIJKя", " ABCDEFGHIJK"} {
		if ValidShortID(id) {
			t.Fatal("invalid Site ID accepted", id)
		}
	}
	for _, raw := range []string{"https://www.EXAMPLE.com./a", "http://example.com:8080/b?q=1"} {
		host, _, err := Page(raw)
		if err != nil || SiteID(host) != "U368FL4X4rW8" {
			t.Fatal("normalization/identity changed", raw, host, err)
		}
	}
	// Include fixed-width values requiring a leading zero, and concurrent calls
	// to the shared modulo constant. This is a sample check, not a collision proof.
	seen := map[string]string{}
	zeroPadded := false
	for i := 0; i < 2000; i++ {
		host := fmt.Sprintf("sample-%d.example", i)
		id := SiteID(host)
		if old, exists := seen[id]; exists {
			t.Fatal("sample collision", host, old)
		}
		seen[id] = host
		if id[0] == '0' {
			zeroPadded = true
		}
		if !ValidShortID(id) {
			t.Fatal("variable-width ID", id)
		}
	}
	if !zeroPadded {
		t.Fatal("padding branch not exercised")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if SiteID("example.com") != "U368FL4X4rW8" {
					t.Error("concurrent ID changed")
				}
			}
		}()
	}
	wg.Wait()
}
