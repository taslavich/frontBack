package stats

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	gotUserID string
	gotReq    QueryRequest
	resp      QueryResponse
	err       error
	closed    bool

	gotTrafficReq  TrafficSegmentRequest
	calculatorResp CalculatorResponse
	recommendResp  RecommendBidResponse
	trafficErr     error

	cumulativeSpendResp   []CumulativeSpendTotal
	cumulativeSpendErr    error
	popRecoveredSpendResp []POPRecoveredSpendTotal
	popRecoveredSpendErr  error
	popRecoveryEventsResp []POPRecoveryEvent
	popRecoveryEventsErr  error
	gotRecoveryCursor     POPRecoveryCursor
	gotRecoveryLimit      int
}

func (f *fakeRepository) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	f.gotUserID = userID
	f.gotReq = req
	return f.resp, f.err
}

func (f *fakeRepository) Calculator(ctx context.Context, req TrafficSegmentRequest) (CalculatorResponse, error) {
	f.gotTrafficReq = req
	return f.calculatorResp, f.trafficErr
}

func (f *fakeRepository) RecommendBid(ctx context.Context, req TrafficSegmentRequest) (RecommendBidResponse, error) {
	f.gotTrafficReq = req
	return f.recommendResp, f.trafficErr
}

func (f *fakeRepository) CumulativeSpend(ctx context.Context) ([]CumulativeSpendTotal, error) {
	return f.cumulativeSpendResp, f.cumulativeSpendErr
}

func (f *fakeRepository) CumulativePOPRecoveredSpend(ctx context.Context) ([]POPRecoveredSpendTotal, error) {
	return f.popRecoveredSpendResp, f.popRecoveredSpendErr
}

func (f *fakeRepository) POPRecoveryEventsAfter(ctx context.Context, cursor POPRecoveryCursor, limit int) ([]POPRecoveryEvent, error) {
	f.gotRecoveryCursor = cursor
	f.gotRecoveryLimit = limit
	return f.popRecoveryEventsResp, f.popRecoveryEventsErr
}

func (f *fakeRepository) Close() error {
	f.closed = true
	return nil
}

func TestServiceQueryDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		resp: QueryResponse{
			Rows: map[string]Summary{
				"DE": {Impressions: 10, Clicks: 1, Spent: 0.5, CTR: 10},
			},
			Totals: Summary{Impressions: 10, Clicks: 1, Spent: 0.5, CTR: 10},
		},
	}
	svc := NewServiceWithRepository(repo)
	req := QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByCountry}

	resp, err := svc.Query(context.Background(), testUserID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotUserID != testUserID {
		t.Fatalf("unexpected userID: %s", repo.gotUserID)
	}
	if repo.gotReq.GroupBy != GroupByCountry {
		t.Fatalf("unexpected req: %#v", repo.gotReq)
	}
	if resp.Totals.Impressions != 10 || resp.Rows["DE"].Clicks != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestServiceQueryReturnsRepositoryError(t *testing.T) {
	expected := errors.New("clickhouse down")
	svc := NewServiceWithRepository(&fakeRepository{err: expected})

	_, err := svc.Query(context.Background(), testUserID, QueryRequest{GroupBy: GroupByCampaign})
	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}

func TestServiceCloseDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewServiceWithRepository(repo)

	if err := svc.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.closed {
		t.Fatal("expected repository to be closed")
	}
}

func TestServiceCalculatorDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		calculatorResp: CalculatorResponse{PotentialImpressions: 123},
	}
	svc := NewServiceWithRepository(repo)
	req := TrafficSegmentRequest{FormatType: "banner", TrafficType: "mainstream"}

	resp, err := svc.Calculator(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotTrafficReq.FormatType != "banner" || resp.PotentialImpressions != 123 {
		t.Fatalf("unexpected delegation: req=%#v resp=%#v", repo.gotTrafficReq, resp)
	}
}

func TestServiceRecommendBidDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		recommendResp: RecommendBidResponse{AverageBid: 0.25},
	}
	svc := NewServiceWithRepository(repo)
	req := TrafficSegmentRequest{FormatType: "push", TrafficType: "adult"}

	resp, err := svc.RecommendBid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotTrafficReq.TrafficType != "adult" || resp.AverageBid != 0.25 {
		t.Fatalf("unexpected delegation: req=%#v resp=%#v", repo.gotTrafficReq, resp)
	}
}

func TestServiceCumulativeSpendDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		cumulativeSpendResp: []CumulativeSpendTotal{
			{EntityType: "campaign", EntityID: testUserID, Amount: "1.25"},
		},
	}
	svc := NewServiceWithRepository(repo)

	totals, err := svc.CumulativeSpend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(totals) != 1 || totals[0].Amount != "1.25" {
		t.Fatalf("unexpected totals: %#v", totals)
	}
}

func TestServiceCumulativePOPRecoveredSpendDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		popRecoveredSpendResp: []POPRecoveredSpendTotal{
			{UserID: testUserID, CampaignID: "22222222-2222-4222-8222-222222222222", Amount: "2.5"},
		},
	}
	svc := NewServiceWithRepository(repo)

	totals, err := svc.CumulativePOPRecoveredSpend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(totals) != 1 || totals[0].Amount != "2.5" {
		t.Fatalf("unexpected recovered totals: %#v", totals)
	}
}

func TestServicePOPRecoveryEventsAfterDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{
		popRecoveryEventsResp: []POPRecoveryEvent{{
			RecoveryAtMS: 1234, SourceKey: "pop-click:abc", UserID: testUserID,
			CampaignID: "22222222-2222-4222-8222-222222222222", Amount: "2.5",
		}},
	}
	svc := NewServiceWithRepository(repo)
	cursor := POPRecoveryCursor{RecoveryAtMS: 1000, SourceKey: "pop-click:old"}
	events, err := svc.POPRecoveryEventsAfter(context.Background(), cursor, 77)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 || events[0].Amount != "2.5" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if repo.gotRecoveryCursor != cursor || repo.gotRecoveryLimit != 77 {
		t.Fatalf("unexpected delegation: cursor=%#v limit=%d", repo.gotRecoveryCursor, repo.gotRecoveryLimit)
	}
}
