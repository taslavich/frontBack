package partners

import "context"

type Stats struct {
	Partner     string  `json:"partner"`
	Advertisers int64   `json:"advertisers"`
	Turnover    float64 `json:"turnover"`
	Withdrawn   float64 `json:"withdrawn"`
}

type statsRepository interface {
	Stats(ctx context.Context, userID string) (Stats, error)
}

type Service struct {
	repo statsRepository
}

func NewService(repo statsRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Stats(ctx context.Context, userID string) (Stats, error) {
	return s.repo.Stats(ctx, userID)
}
