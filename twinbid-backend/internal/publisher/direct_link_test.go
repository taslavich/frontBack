package publisher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	c "twinbid-backend/internal/pubcontract"
	"twinbid-backend/internal/publisherconfig"
)

func TestDirectLinkOutputAndDisabledCreation(t *testing.T) {
	s := &Service{Config: publisherconfig.Config{PublicURL: "https://pub.example.test/"}}
	row := Code{Code: c.Code{Format: "direct_link", ShortCode: "tb_test"}}
	if got := s.codeResponse(row).DirectLinkURL; got != "https://pub.example.test/go/tb_test" {
		t.Fatal(got)
	}
	s.Config.DirectLinkPublicURL = "https://direct.example.test/"
	if got := s.codeResponse(row).DirectLinkURL; got != "https://direct.example.test/go/tb_test" {
		t.Fatal(got)
	}
	row.Format = "popunder"
	b, err := json.Marshal(s.codeResponse(row))
	if err != nil || strings.Contains(string(b), "direct_link_url") {
		t.Fatalf("unexpected Popunder response: %s, %v", b, err)
	}
	if _, err := s.insertCode(context.Background(), "user", c.Code{Format: "direct_link"}); err == nil {
		t.Fatal("Direct Link was accepted without the rollout flag")
	}
}

func TestDirectLinkCabinetLifecycle(t *testing.T) {
	s, h, owner, other := fixture(t)
	s.Config.PublicURL = "https://pub.example.test"
	path := "/api/publisher/ad-codes"
	body := map[string]any{"format": "direct_link", "name": "Download", "traffic_type": "mainstream", "device": "all"}
	if w := req(h, owner, "POST", path, body); w.Code != 409 {
		t.Fatalf("disabled creation: %d %s", w.Code, w.Body)
	}
	s.Config.DirectLinkEnabled = true
	if w := req(h, owner, "GET", path, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"supported_formats":["popunder","direct_link"]`) {
		t.Fatalf("formats: %d %s", w.Code, w.Body)
	}
	w := req(h, owner, "POST", path, body)
	var out struct{ Data Code }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 ||
		out.Data.ID == "" || out.Data.Format != "direct_link" ||
		out.Data.DirectLinkURL != "https://pub.example.test/go/"+out.Data.ShortCode {
		t.Fatalf("create: %d %s (%v)", w.Code, w.Body, err)
	}
	v := out.Data
	pop := req(h, owner, "POST", path, map[string]any{
		"format": "popunder", "name": "Existing installation", "traffic_type": "adult", "device": "all",
	})
	var popOut struct{ Data Code }
	if err := json.Unmarshal(pop.Body.Bytes(), &popOut); err != nil || pop.Code != 200 ||
		popOut.Data.Format != "popunder" || popOut.Data.ShortCode == "" ||
		popOut.Data.ShortCode == v.ShortCode || strings.Contains(pop.Body.String(), "direct_link_url") {
		t.Fatalf("Popunder must remain a separate script-only code: %d %s (%v)", pop.Code, pop.Body, err)
	}
	list := req(h, owner, "GET", path, nil)
	var listed struct{ Data struct{ Items []Code } }
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || list.Code != 200 || len(listed.Data.Items) != 2 {
		t.Fatalf("list: %d %s (%v)", list.Code, list.Body, err)
	}
	for _, code := range listed.Data.Items {
		if (code.ID == v.ID && code.DirectLinkURL != v.DirectLinkURL) ||
			(code.ID == popOut.Data.ID && code.DirectLinkURL != "") {
			t.Fatalf("incorrect installation in list: %+v", code)
		}
	}
	target := path + "/" + v.ID
	if w := req(h, other, "GET", target, nil); w.Code != 404 {
		t.Fatalf("owner isolation: %d %s", w.Code, w.Body)
	}
	for _, method := range []string{"GET", "PATCH"} {
		w := req(h, owner, method, target, map[string]any{"name": "Changed", "version": v.Version})
		if w.Code != 200 || !strings.Contains(w.Body.String(), v.DirectLinkURL) {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body)
		}
	}
	w = req(h, owner, "POST", target+"/duplicate", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 ||
		out.Data.ID == v.ID || out.Data.DirectLinkURL == v.DirectLinkURL || out.Data.DirectLinkURL == "" {
		t.Fatalf("duplicate: %d %s (%v)", w.Code, w.Body, err)
	}
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := s.Redis.HGet(context.Background(), c.CodeKey(v.ShortCode), "data").Bytes()
	var synced c.Code
	if err != nil || json.Unmarshal(data, &synced) != nil || synced.Format != "direct_link" || synced.ID != v.ID {
		t.Fatalf("sync: %s (%v)", data, err)
	}
}
