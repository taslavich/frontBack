package creatives

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"twinbid-backend/internal/campaigns"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/storage"

	"github.com/google/uuid"
)

type Service struct {
	repo        *Repository
	campaignSvc *campaigns.Service
	s3          *storage.S3Storage
}

func NewService(repo *Repository, campaignSvc *campaigns.Service, s3 *storage.S3Storage) *Service {
	return &Service{repo: repo, campaignSvc: campaignSvc, s3: s3}
}

func (s *Service) ListByCampaign(ctx context.Context, userID, campaignID string) ([]models.Creative, error) {
	items, err := s.repo.ListByCampaign(ctx, userID, campaignID)
	if err != nil {
		return nil, err
	}
	return s.withPresignedURLs(ctx, items)
}

func (s *Service) Create(ctx context.Context, userID, campaignID string, req FormCreativeRequest, file multipart.File, header *multipart.FileHeader, filename string) (models.Creative, error) {
	format, err := s.campaignSvc.GetFormat(ctx, campaignID)
	if err != nil {
		return models.Creative{}, err
	}
	creativeID := uuid.NewString()
	cr := models.Creative{ID: creativeID, CampaignID: campaignID, CreativeName: req.CreativeName, Link: req.Link, TrackersMacros: nonNilMap(req.TrackersMacros), W: req.W, H: req.H, Title: req.Title, Description: req.Description, FormatType: format}
	if err := s.validate(format, cr, file, false); err != nil {
		return models.Creative{}, err
	}
	if format != "popunder" {
		key, name, ext, err := s.upload(ctx, campaignID, creativeID, file, header, filename)
		if err != nil {
			return models.Creative{}, err
		}
		cr.Name = &name
		cr.S3FilePath = &key
		cr.FileFormat = &ext
	}
	created, err := s.repo.Create(ctx, cr)
	if err != nil {
		if cr.S3FilePath != nil {
			_ = s.s3.Delete(ctx, *cr.S3FilePath)
		}
		return models.Creative{}, err
	}
	return s.withPresignedURL(ctx, created)
}

func (s *Service) Patch(ctx context.Context, userID, creativeID string, req FormCreativeRequest, hasReq bool, file multipart.File, header *multipart.FileHeader, filename string) (models.Creative, error) {
	current, err := s.repo.Get(ctx, userID, creativeID)
	if err != nil {
		return models.Creative{}, err
	}
	if hasReq {
		if req.CreativeName != "" {
			current.CreativeName = req.CreativeName
		}
		if req.Link != "" {
			current.Link = req.Link
		}
		if req.TrackersMacros != nil {
			current.TrackersMacros = req.TrackersMacros
		}
		if req.W != nil {
			current.W = req.W
		}
		if req.H != nil {
			current.H = req.H
		}
		if req.Title != nil {
			current.Title = req.Title
		}
		if req.Description != nil {
			current.Description = req.Description
		}
	}
	if err := s.validate(current.FormatType, current, file, true); err != nil {
		return models.Creative{}, err
	}
	var oldKey *string
	if file != nil {
		key, name, ext, err := s.upload(ctx, current.CampaignID, current.ID, file, header, filename)
		if err != nil {
			return models.Creative{}, err
		}
		oldKey = current.S3FilePath
		current.Name = &name
		current.S3FilePath = &key
		current.FileFormat = &ext
	}
	updated, err := s.repo.Update(ctx, userID, current)
	if err != nil {
		if file != nil && current.S3FilePath != nil {
			_ = s.s3.Delete(ctx, *current.S3FilePath)
		}
		return models.Creative{}, err
	}
	if oldKey != nil && *oldKey != "" && (updated.S3FilePath == nil || *oldKey != *updated.S3FilePath) {
		_ = s.s3.Delete(ctx, *oldKey)
	}
	return s.withPresignedURL(ctx, updated)
}

func (s *Service) Delete(ctx context.Context, userID, creativeID string) error {
	cr, err := s.repo.Delete(ctx, userID, creativeID)
	if err != nil {
		return err
	}
	if cr.S3FilePath != nil {
		return s.s3.Delete(ctx, *cr.S3FilePath)
	}
	return nil
}

func (s *Service) validate(format string, cr models.Creative, file io.Reader, patch bool) error {
	if cr.CreativeName == "" {
		return httpx.BadRequest("creative_name is required")
	}
	if cr.Link == "" {
		return httpx.BadRequest("link is required")
	}
	if format == "popunder" {
		return nil
	}
	if !patch && file == nil {
		return httpx.BadRequest("file is required for banner/native/push creatives")
	}
	if format == "banner" {
		if cr.W == nil || cr.H == nil {
			return httpx.BadRequest("w and h are required for banner creatives")
		}
	}
	if format == "native" || format == "push" {
		if cr.Title == nil || *cr.Title == "" {
			return httpx.BadRequest("title is required")
		}
		if cr.Description == nil || *cr.Description == "" {
			return httpx.BadRequest("description is required")
		}
	}
	return nil
}

func (s *Service) upload(ctx context.Context, campaignID, creativeID string, file multipart.File, header *multipart.FileHeader, filename string) (key, name, ext string, err error) {
	if filename == "" && header != nil {
		filename = header.Filename
	}
	if filename == "" {
		filename = "creative"
	}
	name = filepath.Base(filename)
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	contentType := "application/octet-stream"
	if header != nil && header.Header.Get("Content-Type") != "" {
		contentType = header.Header.Get("Content-Type")
	}
	key = fmt.Sprintf("creatives/%s/%s/%s", campaignID, creativeID, name)
	if err := s.s3.Put(ctx, key, contentType, file); err != nil {
		return "", "", "", err
	}
	return key, name, ext, nil
}

func (s *Service) withPresignedURLs(ctx context.Context, items []models.Creative) ([]models.Creative, error) {
	for i := range items {
		item, err := s.withPresignedURL(ctx, items[i])
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func (s *Service) withPresignedURL(ctx context.Context, cr models.Creative) (models.Creative, error) {
	if cr.S3FilePath == nil || *cr.S3FilePath == "" {
		return cr, nil
	}
	url, err := s.s3.PresignGet(ctx, *cr.S3FilePath)
	if err != nil {
		return models.Creative{}, err
	}
	cr.PresignedS3URL = &url
	return cr, nil
}

func nonNilMap(v models.TargetingMap) models.TargetingMap {
	if v == nil {
		return models.TargetingMap{}
	}
	return v
}
