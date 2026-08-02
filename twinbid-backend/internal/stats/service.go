package stats

import (
	"context"

	"twinbid-backend/internal/config"
)

type querier interface {
	Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error)
}

type trafficQuerier interface {
	Calculator(ctx context.Context, req TrafficSegmentRequest) (CalculatorResponse, error)
	RecommendBid(ctx context.Context, req TrafficSegmentRequest) (RecommendBidResponse, error)
}

type cumulativeSpendQuerier interface {
	CumulativeSpend(ctx context.Context) ([]CumulativeSpendTotal, error)
}

type closer interface {
	Close() error
}

type repository interface {
	querier
	trafficQuerier
	cumulativeSpendQuerier
	closer
}

type Service struct {
	repo repository
}

func NewService(ctx context.Context, cfg config.ClickHouseConfig) (*Service, error) {
	repo, err := NewClickHouseRepository(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo}, nil
}

func NewServiceWithRepository(repo repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	return s.repo.Query(ctx, userID, req)
}

func (s *Service) Calculator(ctx context.Context, req TrafficSegmentRequest) (CalculatorResponse, error) {
	return s.repo.Calculator(ctx, req)
}

func (s *Service) RecommendBid(ctx context.Context, req TrafficSegmentRequest) (RecommendBidResponse, error) {
	return s.repo.RecommendBid(ctx, req)
}

func (s *Service) CumulativeSpend(ctx context.Context) ([]CumulativeSpendTotal, error) {
	return s.repo.CumulativeSpend(ctx)
}

func (s *Service) Close() error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.Close()
}
