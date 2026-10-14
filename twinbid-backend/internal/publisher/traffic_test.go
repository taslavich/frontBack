package publisher

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	c "twinbid-backend/internal/pubcontract"
)

func TestCreateRequiresExplicitTrafficType(t *testing.T) {
	s := &Service{}
	for _, body := range []string{`{"name":"a"}`, `{"name":"a","traffic_type":null}`, `{"name":"a","traffic_type":""}`, `{"name":"a","traffic_type":"mixed"}`, `{"name":"a","traffic_type":"other"}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/publisher/ad-codes", strings.NewReader(body))
		s.createCode(w, r)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "traffic_type") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestUpdateOptionalTrafficType(t *testing.T) {
	s, h, user, other := fixture(t)
	code, e := s.insertCode(context.Background(), user, c.Code{Name: "code", Format: "popunder", TrafficType: "adult", Device: "all", TriggerClick: 1, FrequencyMinutes: 1})
	if e != nil {
		t.Fatal(e)
	}
	assertStored := func(name, traffic string, version int64) {
		t.Helper()
		var gotName, gotTraffic string
		var gotVersion int64
		if e := s.DB.QueryRow(`SELECT name,traffic_type,version FROM ad_codes WHERE ad_code_id=$1`, code.ID).Scan(&gotName, &gotTraffic, &gotVersion); e != nil {
			t.Fatal(e)
		}
		if gotName != name || gotTraffic != traffic || gotVersion != version {
			t.Fatal(gotName, gotTraffic, gotVersion)
		}
	}
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 1, "name": "renamed"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertStored("renamed", "adult", 2)
	for _, value := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`""`), json.RawMessage(`"mixed"`), json.RawMessage(`"invalid"`), json.RawMessage(`true`), json.RawMessage(`123`), json.RawMessage(`[]`), json.RawMessage(`{}`)} {
		body := map[string]any{"version": 2, "name": "must not change", "traffic_type": value}
		if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
		assertStored("renamed", "adult", 2)
	}
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 2, "traffic_type": "mainstream"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertStored("renamed", "mainstream", 3)
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 3, "traffic_type": "adult"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertStored("renamed", "adult", 4)
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 3, "name": "stale"}); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := req(h, other, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 4, "name": "foreign"}); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertStored("renamed", "adult", 4)
}

func TestExternalCreditsAreDisabled(t *testing.T) {
	s := &Service{}
	w := httptest.NewRecorder()
	s.Financial(w, httptest.NewRequest("POST", "/api/internal/publisher/financial-events", strings.NewReader(`{}`)))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "agg_stats") {
		t.Fatal(w.Code, w.Body.String())
	}
}
