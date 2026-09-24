package stats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"twinbid-backend/internal/httpx"
)

const (
	testAdvertiserToken = "adv_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testAdvertiserUser  = "11111111-1111-1111-1111-111111111111"
	testAdvertiserCID   = "22222222-2222-2222-2222-222222222222"
)

type fakeAdvertiserAPIRepository struct {
	userID     string
	consumeErr error
	owned      bool
	ownedErr   error
}

func (f *fakeAdvertiserAPIRepository) ConsumeAdvertiserToken(context.Context, string) (string, error) {
	if f.consumeErr != nil {
		return "", f.consumeErr
	}
	return f.userID, nil
}

func (f *fakeAdvertiserAPIRepository) CampaignBelongsToUser(context.Context, string, string) (bool, error) {
	return f.owned, f.ownedErr
}

type fakeAdvertiserStatsQuerier struct {
	called bool
	userID string
	req    QueryRequest
	res    QueryResponse
	err    error
}

func (f *fakeAdvertiserStatsQuerier) Query(_ context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	f.called = true
	f.userID = userID
	f.req = req
	return f.res, f.err
}

func TestAdvertiserAPIQueryScopesStatsToTokenOwnerAndCampaign(t *testing.T) {
	repo := &fakeAdvertiserAPIRepository{userID: testAdvertiserUser, owned: true}
	statsSvc := &fakeAdvertiserStatsQuerier{res: QueryResponse{Rows: map[string]Summary{}}}
	h := newAdvertiserAPIHandlerWithRepository(repo, statsSvc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/advertiser/stats?token="+testAdvertiserToken+"&campaign_id="+testAdvertiserCID+"&from=2026-09-01&to=2026-09-24&group_by=date", nil)
	rec := httptest.NewRecorder()
	h.Query(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !statsSvc.called {
		t.Fatal("stats service was not called")
	}
	if statsSvc.userID != testAdvertiserUser {
		t.Fatalf("wrong user scope: %q", statsSvc.userID)
	}
	if len(statsSvc.req.CampaignIDs) != 1 || statsSvc.req.CampaignIDs[0] != testAdvertiserCID {
		t.Fatalf("wrong campaign scope: %#v", statsSvc.req.CampaignIDs)
	}
	if statsSvc.req.GroupBy != GroupByDate || statsSvc.req.From != "2026-09-01" || statsSvc.req.To != "2026-09-24" {
		t.Fatalf("unexpected stats request: %#v", statsSvc.req)
	}
}

func TestAdvertiserAPIQueryHidesForeignCampaign(t *testing.T) {
	repo := &fakeAdvertiserAPIRepository{userID: testAdvertiserUser, owned: false}
	statsSvc := &fakeAdvertiserStatsQuerier{}
	h := newAdvertiserAPIHandlerWithRepository(repo, statsSvc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/advertiser/stats?token="+testAdvertiserToken+"&campaign_id="+testAdvertiserCID+"&from=2026-09-01&to=2026-09-24&group_by=country", nil)
	rec := httptest.NewRecorder()
	h.Query(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if statsSvc.called {
		t.Fatal("stats service must not be called for a foreign campaign")
	}
}

func TestAdvertiserAPIQueryRateLimit(t *testing.T) {
	repo := &fakeAdvertiserAPIRepository{consumeErr: httpx.HTTPError{
		Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "limited",
	}}
	h := newAdvertiserAPIHandlerWithRepository(repo, &fakeAdvertiserStatsQuerier{})

	req := httptest.NewRequest(http.MethodGet,
		"/api/advertiser/stats?token="+testAdvertiserToken+"&campaign_id="+testAdvertiserCID+"&from=2026-09-01&to=2026-09-24&group_by=date", nil)
	rec := httptest.NewRecorder()
	h.Query(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After=1, got %q", got)
	}
}

func TestRedactAdvertiserTokenQuery(t *testing.T) {
	var seenURI, seenToken string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURI = r.RequestURI
		seenToken = advertiserTokenFromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/advertiser/stats?token="+testAdvertiserToken+"&campaign_id="+testAdvertiserCID, nil)
	rec := httptest.NewRecorder()
	RedactAdvertiserTokenQuery(next).ServeHTTP(rec, req)

	if strings.Contains(seenURI, testAdvertiserToken) {
		t.Fatalf("raw token leaked into downstream RequestURI: %q", seenURI)
	}
	if seenToken != testAdvertiserToken {
		t.Fatalf("handler lost raw token: %q", seenToken)
	}
}
