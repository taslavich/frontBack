package stats

import (
	"context"

	"twinbid-backend/internal/config"
)

type querier interface {
	Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error)
}

type closer interface {
	Close() error
}

type repository interface {
	querier
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

func (s *Service) Close() error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.Close()
}
