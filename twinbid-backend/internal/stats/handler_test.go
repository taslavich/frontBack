package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/config"
)

type fakeQueryService struct {
	gotUserID string
	gotReq    QueryRequest
	resp      QueryResponse
	err       error

	gotTrafficReq  TrafficSegmentRequest
	calculatorResp CalculatorResponse
	recommendResp  RecommendBidResponse
	trafficErr     error
}

func (f *fakeQueryService) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	f.gotUserID = userID
	f.gotReq = req
	return f.resp, f.err
}

func (f *fakeQueryService) Calculator(ctx context.Context, req TrafficSegmentRequest) (CalculatorResponse, error) {
	f.gotTrafficReq = req
	return f.calculatorResp, f.trafficErr
}

func (f *fakeQueryService) RecommendBid(ctx context.Context, req TrafficSegmentRequest) (RecommendBidResponse, error) {
	f.gotTrafficReq = req
	return f.recommendResp, f.trafficErr
}

func TestHandlerQueryOK(t *testing.T) {
	svc := &fakeQueryService{
		resp: QueryResponse{
			Rows: map[string]Summary{
				"2026-05-06": {Impressions: 100, Clicks: 5, Spent: 1.25, CTR: 5},
			},
			Totals: Summary{Impressions: 100, Clicks: 5, Spent: 1.25, CTR: 5},
		},
	}

	body := []byte(`{
		"from":"2026-05-01",
		"to":"2026-05-06",
		"group_by":"date",
		"campaign_ids":["22222222-2222-2222-2222-222222222222"],
		"creative_ids":["33333333-3333-3333-3333-333333333333"],
		"filters":{"country":["DE"]}
	}`)

	w := performStatsRequest(t, NewHandler(svc), body, true)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.gotUserID != testUserID {
		t.Fatalf("unexpected userID: %s", svc.gotUserID)
	}
	if svc.gotReq.GroupBy != GroupByDate || svc.gotReq.Filters["country"][0] != "DE" {
		t.Fatalf("unexpected request passed to service: %#v", svc.gotReq)
	}

	var envelope struct {
		Success  bool          `json:"success"`
		ErrorMsg string        `json:"errorMsg"`
		Data     QueryResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !envelope.Success || envelope.ErrorMsg != "" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.Data.Totals.Impressions != 100 || envelope.Data.Rows["2026-05-06"].Clicks != 5 {
		t.Fatalf("unexpected data: %#v", envelope.Data)
	}
}

func TestHandlerQueryBadJSON(t *testing.T) {
	svc := &fakeQueryService{}
	w := performStatsRequest(t, NewHandler(svc), []byte(`{bad json`), true)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerQueryUnauthorizedWithoutToken(t *testing.T) {
	svc := &fakeQueryService{}
	w := performStatsRequest(t, NewHandler(svc), []byte(`{"from":"2026-05-01","to":"2026-05-06","group_by":"date"}`), false)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerQueryServiceError(t *testing.T) {
	svc := &fakeQueryService{err: errors.New("boom")}
	w := performStatsRequest(t, NewHandler(svc), []byte(`{"from":"2026-05-01","to":"2026-05-06","group_by":"date"}`), true)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerCalculatorOK(t *testing.T) {
	svc := &fakeQueryService{
		calculatorResp: CalculatorResponse{PotentialImpressions: 128400},
	}
	body := []byte(`{
		"format_type":"banner",
		"traffic_type":"mainstream",
		"country":["DE"],
		"country_mode":"include"
	}`)

	w := performTrafficRequest(t, NewHandler(svc), "/api/calculator", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.gotTrafficReq.FormatType != "banner" || svc.gotTrafficReq.Country[0] != "DE" {
		t.Fatalf("unexpected request: %#v", svc.gotTrafficReq)
	}

	var envelope struct {
		Success bool               `json:"success"`
		Data    CalculatorResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !envelope.Success || envelope.Data.PotentialImpressions != 128400 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestHandlerRecommendBidOK(t *testing.T) {
	svc := &fakeQueryService{
		recommendResp: RecommendBidResponse{AverageBid: 1.24},
	}
	body := []byte(`{
		"format_type":"popunder",
		"traffic_type":"mixed",
		"browser":["Chrome"],
		"browser_mode":"exclude"
	}`)

	w := performTrafficRequest(t, NewHandler(svc), "/api/recommend_bid", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Success bool                 `json:"success"`
		Data    RecommendBidResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !envelope.Success || envelope.Data.AverageBid != 1.24 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestHandlerCalculatorBadJSON(t *testing.T) {
	w := performTrafficRequest(t, NewHandler(&fakeQueryService{}), "/api/calculator", []byte(`{bad json`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func performTrafficRequest(t *testing.T, h *Handler, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	const secret = "test-secret"
	authSvc := auth.NewService(nil, config.JWTConfig{Secret: secret}, config.SMTPConfig{})

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Post("/api/calculator", h.Calculator)
		r.Post("/api/recommend_bid", h.RecommendBid)
	})

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	token, _, err := auth.GenerateJWT(secret, testUserID, "test@example.com", "access", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func performStatsRequest(t *testing.T, h *Handler, body []byte, withToken bool) *httptest.ResponseRecorder {
	t.Helper()

	const secret = "test-secret"
	authSvc := auth.NewService(nil, config.JWTConfig{Secret: secret}, config.SMTPConfig{})

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Post("/api/stats/query", h.Query)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/stats/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if withToken {
		token, _, err := auth.GenerateJWT(secret, testUserID, "test@example.com", "access", time.Hour)
		if err != nil {
			t.Fatalf("token: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
