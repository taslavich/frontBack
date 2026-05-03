package notifications

import (
	"context"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

var validType = map[string]bool{"incomplete_topup": true, "low_balance": true, "campaign_status": true, "other": true}
var validStatus = map[string]bool{"active": true, "inactive": true}

func (s *Service) List(ctx context.Context, userID, status string) ([]models.Notification, error) {
	if status != "" && !validStatus[status] {
		return nil, httpx.BadRequest("invalid status")
	}
	/*if status == "" || status == "active" {
		if err := s.repo.EnsureAutomatic(ctx, userID); err != nil {
			return nil, err
		}
	}*/
	return s.repo.List(ctx, userID, "active")
}

func (s *Service) Create(ctx context.Context, userID string, req CreateNotificationRequest) (models.Notification, error) {
	if req.Text == "" {
		return models.Notification{}, httpx.BadRequest("text is required")
	}
	typ := req.Type
	if typ == "" {
		typ = "other"
	}
	if !validType[typ] {
		return models.Notification{}, httpx.BadRequest("invalid type")
	}
	n := models.Notification{UserID: userID, TransactionID: req.TransactionID, CampaignID: req.CampaignID, DepositAmount: req.DepositAmount, Status: "active", Text: req.Text, Type: typ}
	return s.repo.Create(ctx, n)
}

func (s *Service) Patch(ctx context.Context, userID, id string, req PatchNotificationRequest) (models.Notification, error) {
	if req.Status != nil && !validStatus[*req.Status] {
		return models.Notification{}, httpx.BadRequest("invalid status")
	}
	if req.Type != nil && !validType[*req.Type] {
		return models.Notification{}, httpx.BadRequest("invalid type")
	}
	return s.repo.Patch(ctx, userID, id, req)
}
