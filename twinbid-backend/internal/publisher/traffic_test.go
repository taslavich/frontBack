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
	for _, body := range []string{`{"name":"a"}`, `{"name":"a","traffic_type":null}`, `{"name":"a","traffic_type":"mixed"}`, `{"name":"a","traffic_type":"other"}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/publisher/ad-codes", strings.NewReader(body))
		s.createCode(w, r)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "traffic_type") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestUpdateRequiresTrafficType(t *testing.T) {
	s, h, user, _ := fixture(t)
	code, e := s.insertCode(context.Background(), user, c.Code{Name: "code", Format: "popunder", TrafficType: "adult", Device: "all", TriggerClick: 1, FrequencyMinutes: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, value := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`"mixed"`), json.RawMessage(`"invalid"`)} {
		body := map[string]any{"version": 1}
		if value != nil {
			body["traffic_type"] = value
		}
		if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := req(h, user, "PATCH", "/api/publisher/ad-codes/"+code.ID, map[string]any{"version": 1, "traffic_type": "mainstream"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestExternalCreditsAreDisabled(t *testing.T) {
	s := &Service{}
	w := httptest.NewRecorder()
	s.Financial(w, httptest.NewRequest("POST", "/api/internal/publisher/financial-events", strings.NewReader(`{}`)))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "agg_stats") {
		t.Fatal(w.Code, w.Body.String())
	}
}
