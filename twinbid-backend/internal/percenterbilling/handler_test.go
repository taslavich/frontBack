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
}

func (f *fakePromoApplier) ApplyPromoSpend(_ context.Context, req ApplyRequest) (PromoState, error) {
	f.got = req
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
}
