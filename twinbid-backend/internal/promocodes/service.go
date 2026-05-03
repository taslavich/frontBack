package promocodes

import (
	"context"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }
func (s *Service) GetByCode(ctx context.Context, code string) (models.Promocode, error) {
	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return models.Promocode{}, err
	}
	now := time.Now().UTC()
	if p.ValidFrom != nil && now.Before(*p.ValidFrom) {
		return models.Promocode{}, httpx.BadRequest("promocode is not active yet")
	}
	if p.ValidTo != nil && now.After(*p.ValidTo) {
		return models.Promocode{}, httpx.BadRequest("promocode expired")
	}
	if p.UsageLimit != nil && p.UsageCount >= *p.UsageLimit {
		return models.Promocode{}, httpx.BadRequest("promocode usage limit exceeded")
	}
	return p, nil
}
