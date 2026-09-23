package percenterbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePromoApplier struct {
	got ApplyRequest
	err error
}

func (f *fakePromoApplier) ApplyPromoSpend(_ context.Context, req ApplyRequest) (PromoState, error) {
	f.got = req
	if f.err != nil {
		return PromoState{}, f.err
	}
	return PromoState{Remaining: 4.5, Revision: 9}, nil
}

func TestHandlerApplyRequiresInternalSecretAndReturnsState(t *testing.T) {
	fake := &fakePromoApplier{}
	h := NewHandler(fake, "secret")
	payload := ApplyRequest{EventID: "event", UserID: "user", CampaignID: "campaign", SpendDelta: 1.25}
	body, _ := json.Marshal(payload)

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/internal/percenter/promo-spend", bytes.NewReader(body))
	unauthorizedRec := httptest.NewRecorder()
	h.Apply(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorizedRec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/internal/percenter/promo-spend", bytes.NewReader(body))
	req.Header.Set("X-Bot-Secret", "secret")
	rec := httptest.NewRecorder()
	h.Apply(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.got != payload {
		t.Fatalf("unexpected request: %+v", fake.got)
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Remaining float64 `json:"remaining"`
			Revision  int64   `json:"revision"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Remaining != 4.5 || envelope.Data.Revision != 9 {
		t.Fatalf("unexpected promo state response: %+v", envelope)
	}
}

func TestHandlerApplyReturnsConflictForReusedEventIDWithDifferentPayload(t *testing.T) {
	fake := &fakePromoApplier{err: ErrEventPayloadConflict}
	h := NewHandler(fake, "secret")
	body, _ := json.Marshal(ApplyRequest{EventID: "event", UserID: "user", CampaignID: "campaign", SpendDelta: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/internal/percenter/promo-spend", bytes.NewReader(body))
	req.Header.Set("X-Bot-Secret", "secret")
	rec := httptest.NewRecorder()

	h.Apply(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
