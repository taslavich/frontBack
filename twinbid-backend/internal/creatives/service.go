package creatives

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"twinbid-backend/internal/campaigns"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/storage"

	"github.com/google/uuid"
)

const maxCreativeImageSize int64 = 64 << 20

type Service struct {
	repo             *Repository
	campaignSvc      *campaigns.Service
	s3               *storage.S3Storage
	publicAPIBaseURL string
}

func NewService(repo *Repository, campaignSvc *campaigns.Service, s3 *storage.S3Storage, publicAPIBaseURL string) *Service {
	return &Service{
		repo:             repo,
		campaignSvc:      campaignSvc,
		s3:               s3,
		publicAPIBaseURL: strings.TrimRight(strings.TrimSpace(publicAPIBaseURL), "/"),
	}
}

func (s *Service) ListByCampaign(ctx context.Context, userID, campaignID string) ([]models.Creative, error) {
	return s.repo.ListByCampaign(ctx, userID, campaignID)
}

func (s *Service) UploadImage(ctx context.Context, userID, campaignID string, file multipart.File, header *multipart.FileHeader, filename string) (models.CreativeImage, error) {
	campaign, err := s.ownedCampaign(ctx, userID, campaignID)
	if err != nil {
		return models.CreativeImage{}, err
	}
	if campaign.FormatType == "popunder" {
		return models.CreativeImage{}, httpx.BadRequest("popunder creatives do not use images")
	}
	if file == nil || header == nil {
		return models.CreativeImage{}, httpx.BadRequest("file is required")
	}

	sizeBytes, mimeType, extension, err := inspectImage(file)
	if err != nil {
		return models.CreativeImage{}, err
	}
	if filename == "" {
		filename = header.Filename
	}
	filename = sanitizeFilename(filename, extension)

	imageID := uuid.NewString()
	s3Key := fmt.Sprintf("images/%s/%s.%s", campaignID, imageID, extension)
	webURL := s.mediaURL(imageID)
	if webURL == "" {
		return models.CreativeImage{}, fmt.Errorf("PUBLIC_API_BASE_URL is required")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return models.CreativeImage{}, fmt.Errorf("rewind image: %w", err)
	}
	if err := s.s3.Put(ctx, s3Key, mimeType, file); err != nil {
		return models.CreativeImage{}, fmt.Errorf("upload image to s3: %w", err)
	}

	image := models.CreativeImage{
		ID:           imageID,
		UserID:       userID,
		CampaignID:   campaignID,
		S3Key:        s3Key,
		WebURL:       webURL,
		OriginalName: filename,
		MimeType:     mimeType,
		FileFormat:   extension,
		SizeBytes:    sizeBytes,
	}
	created, err := s.repo.CreateImage(ctx, image)
	if err != nil {
		_ = s.s3.Delete(ctx, s3Key)
		return models.CreativeImage{}, err
	}
	return created, nil
}

func (s *Service) Create(ctx context.Context, userID, campaignID string, req CreateCreativeRequest) (models.Creative, error) {
	campaign, err := s.ownedCampaign(ctx, userID, campaignID)
	if err != nil {
		return models.Creative{}, err
	}
	bannerType := normalizeOptionalString(req.BannerType)
	imageID := normalizeOptionalString(req.ImageID)
	creative := models.Creative{
		ID:             uuid.NewString(),
		CampaignID:     campaignID,
		CreativeName:   req.CreativeName,
		ADM:            req.ADM,
		BannerType:     bannerType,
		TrackersMacros: nonNilMacroMap(req.TrackersMacros),
		W:              req.W,
		H:              req.H,
		Title:          req.Title,
		Description:    req.Description,
		ImageID:        imageID,
		FormatType:     campaign.FormatType,
	}
	if err := validateCreative(creative); err != nil {
		return models.Creative{}, err
	}
	return s.repo.Create(ctx, userID, creative, imageID)
}

func (s *Service) Patch(ctx context.Context, userID, creativeID string, req PatchCreativeRequest) (models.Creative, error) {
	current, err := s.repo.Get(ctx, userID, creativeID)
	if err != nil {
		return models.Creative{}, err
	}
	if req.CreativeName != nil {
		current.CreativeName = *req.CreativeName
	}
	if req.ADM != nil {
		current.ADM = *req.ADM
	}
	if req.BannerType != nil {
		current.BannerType = normalizeOptionalString(req.BannerType)
	}
	if req.TrackersMacros != nil {
		current.TrackersMacros = nonNilMacroMap(*req.TrackersMacros)
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

	imageChange := req.ImageID
	if imageChange.Set {
		imageChange.Value = normalizeOptionalString(imageChange.Value)
		current.ImageID = imageChange.Value
	}

	// Formats without an image automatically detach an old image. This is
	// important when a banner is switched from img to iframe.
	if current.FormatType == "popunder" || isIframeBanner(current) {
		if imageChange.Set && imageChange.Value != nil {
			return models.Creative{}, httpx.BadRequest("image_id is not allowed for this creative type")
		}
		if current.ImageID != nil {
			imageChange = OptionalString{Set: true, Value: nil}
			current.ImageID = nil
		}
	}

	if err := validateCreative(current); err != nil {
		return models.Creative{}, err
	}
	updated, detachedImage, err := s.repo.Update(ctx, userID, current, imageChange)
	if err != nil {
		return models.Creative{}, err
	}
	if detachedImage != nil {
		if err := s.deleteDetachedImage(ctx, *detachedImage); err != nil {
			return models.Creative{}, fmt.Errorf("creative updated but old image cleanup failed: %w", err)
		}
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID, creativeID string) error {
	_, image, err := s.repo.Delete(ctx, userID, creativeID)
	if err != nil {
		return err
	}
	if image != nil {
		if err := s.deleteDetachedImage(ctx, *image); err != nil {
			return fmt.Errorf("creative deleted but image cleanup failed: %w", err)
		}
	}
	return nil
}

func (s *Service) GetMediaImage(ctx context.Context, imageID string) (models.CreativeImage, *storage.Object, error) {
	image, err := s.repo.GetImage(ctx, strings.TrimSpace(imageID))
	if err != nil {
		return models.CreativeImage{}, nil, err
	}
	object, err := s.s3.Get(ctx, image.S3Key)
	if err != nil {
		if storage.IsNotFound(err) {
			return models.CreativeImage{}, nil, httpx.NotFound("image object not found")
		}
		return models.CreativeImage{}, nil, err
	}
	return image, object, nil
}

func (s *Service) HeadMediaImage(ctx context.Context, imageID string) (models.CreativeImage, storage.ObjectMetadata, error) {
	image, err := s.repo.GetImage(ctx, strings.TrimSpace(imageID))
	if err != nil {
		return models.CreativeImage{}, storage.ObjectMetadata{}, err
	}
	metadata, err := s.s3.Head(ctx, image.S3Key)
	if err != nil {
		if storage.IsNotFound(err) {
			return models.CreativeImage{}, storage.ObjectMetadata{}, httpx.NotFound("image object not found")
		}
		return models.CreativeImage{}, storage.ObjectMetadata{}, err
	}
	return image, metadata, nil
}

func (s *Service) ownedCampaign(ctx context.Context, userID, campaignID string) (models.Campaign, error) {
	campaign, err := s.campaignSvc.Get(ctx, campaignID)
	if err != nil {
		return models.Campaign{}, err
	}
	if campaign.UserID != userID {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	return campaign, nil
}

func (s *Service) deleteDetachedImage(ctx context.Context, image models.CreativeImage) error {
	if err := s.s3.Delete(ctx, image.S3Key); err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	if err := s.repo.DeleteImageRecord(ctx, image.ID); err != nil {
		return fmt.Errorf("delete image record: %w", err)
	}
	return nil
}

func (s *Service) mediaURL(imageID string) string {
	if s.publicAPIBaseURL == "" {
		return ""
	}
	return s.publicAPIBaseURL + "/api/media/" + imageID
}

func validateCreative(creative models.Creative) error {
	if strings.TrimSpace(creative.CreativeName) == "" {
		return httpx.BadRequest("creative_name is required")
	}
	if strings.TrimSpace(creative.ADM) == "" {
		return httpx.BadRequest("adm is required")
	}
	if creative.W != nil && *creative.W <= 0 {
		return httpx.BadRequest("w must be greater than zero")
	}
	if creative.H != nil && *creative.H <= 0 {
		return httpx.BadRequest("h must be greater than zero")
	}

	switch creative.FormatType {
	case "banner":
		if creative.BannerType == nil {
			return httpx.BadRequest("banner_type is required for banner creatives")
		}
		if *creative.BannerType != "img" && *creative.BannerType != "iframe" {
			return httpx.BadRequest("banner_type must be img or iframe")
		}
		if creative.W == nil || creative.H == nil {
			return httpx.BadRequest("w and h are required for banner creatives")
		}
		if *creative.BannerType == "img" && creative.ImageID == nil {
			return httpx.BadRequest("image_id is required for banner_type=img")
		}
		if *creative.BannerType == "iframe" && creative.ImageID != nil {
			return httpx.BadRequest("image_id is not allowed for banner_type=iframe")
		}
	case "native", "push":
		if creative.BannerType != nil {
			return httpx.BadRequest("banner_type is only allowed for banner creatives")
		}
		if creative.ImageID == nil {
			return httpx.BadRequest("image_id is required for native and push creatives")
		}
		if creative.Title == nil || strings.TrimSpace(*creative.Title) == "" {
			return httpx.BadRequest("title is required")
		}
		if creative.Description == nil || strings.TrimSpace(*creative.Description) == "" {
			return httpx.BadRequest("description is required")
		}
	case "popunder":
		if creative.BannerType != nil {
			return httpx.BadRequest("banner_type is only allowed for banner creatives")
		}
		if creative.ImageID != nil {
			return httpx.BadRequest("image_id is not allowed for popunder creatives")
		}
	default:
		return httpx.BadRequest("unsupported campaign format")
	}
	return nil
}

func inspectImage(file multipart.File) (size int64, mimeType, extension string, err error) {
	size, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, "", "", fmt.Errorf("determine image size: %w", err)
	}
	if size <= 0 {
		return 0, "", "", httpx.BadRequest("image file is empty")
	}
	if size > maxCreativeImageSize {
		return 0, "", "", httpx.BadRequest("image file exceeds 64 MiB")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return 0, "", "", fmt.Errorf("rewind image: %w", err)
	}

	header := make([]byte, 512)
	readBytes, readErr := io.ReadFull(file, header)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return 0, "", "", fmt.Errorf("read image header: %w", readErr)
	}
	mimeType = http.DetectContentType(header[:readBytes])
	extensions := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/gif":  "gif",
		"image/webp": "webp",
	}
	extension, ok := extensions[mimeType]
	if !ok {
		return 0, "", "", httpx.BadRequest("unsupported image type; allowed: jpeg, png, gif, webp")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return 0, "", "", fmt.Errorf("rewind image: %w", err)
	}
	return size, mimeType, extension, nil
}

func sanitizeFilename(filename, extension string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." {
		return "image." + extension
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if strings.TrimSpace(base) == "" {
		base = "image"
	}
	return base + "." + extension
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func isIframeBanner(creative models.Creative) bool {
	return creative.FormatType == "banner" && creative.BannerType != nil && *creative.BannerType == "iframe"
}

func nonNilMacroMap(value models.MacroMap) models.MacroMap {
	if value == nil {
		return models.MacroMap{}
	}
	return value
}
