package creatives

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"twinbid-backend/internal/db"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) ListByCampaign(ctx context.Context, userID, campaignID string) ([]models.Creative, error) {
	rows, err := r.db.QueryContext(ctx, baseCreativeSelect+`
		WHERE cr.campaign_id=$2 AND c.user_id=$1 ORDER BY cr.created_at DESC
	`, userID, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Creative
	for rows.Next() {
		creative, err := scanCreative(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, creative)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, userID, creativeID string) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, baseCreativeSelect+`WHERE cr.id=$2 AND c.user_id=$1`, userID, creativeID)
	creative, err := scanCreative(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Creative{}, httpx.NotFound("creative not found")
	}
	return creative, err
}

func (r *Repository) GetByIDOnly(ctx context.Context, creativeID string) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, baseCreativeSelect+`WHERE cr.id=$1`, creativeID)
	creative, err := scanCreative(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Creative{}, httpx.NotFound("creative not found")
	}
	return creative, err
}

func (r *Repository) Create(ctx context.Context, userID string, creative models.Creative, imageID *string) (models.Creative, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Creative{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO creatives (id, campaign_id, creative_name, adm, banner_type, trackers_macros, w, h, title, description)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
		FROM campaigns c
		WHERE c.campaign_id=$2 AND c.user_id=$11
		RETURNING id
	`, creative.ID, creative.CampaignID, creative.CreativeName, creative.ADM, creative.BannerType,
		jsonArg(creative.TrackersMacros), creative.W, creative.H, creative.Title, creative.Description, userID)
	if err := row.Scan(&creative.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Creative{}, httpx.NotFound("campaign not found")
		}
		return models.Creative{}, err
	}

	if imageID != nil {
		if err := bindImageTx(ctx, tx, userID, creative.CampaignID, creative.ID, *imageID); err != nil {
			return models.Creative{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Creative{}, err
	}
	return r.GetByIDOnly(ctx, creative.ID)
}

// Update updates the creative and atomically changes its image binding.
// The returned old image is detached and can be deleted from S3 after commit.
func (r *Repository) Update(ctx context.Context, userID string, creative models.Creative, imageChange OptionalString) (models.Creative, *models.CreativeImage, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Creative{}, nil, err
	}
	defer tx.Rollback()

	var lockedID string
	if err := tx.QueryRowContext(ctx, `
		SELECT cr.id
		FROM creatives cr
		JOIN campaigns c ON c.campaign_id=cr.campaign_id
		WHERE cr.id=$2 AND c.user_id=$1
		FOR UPDATE OF cr
	`, userID, creative.ID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Creative{}, nil, httpx.NotFound("creative not found")
		}
		return models.Creative{}, nil, err
	}

	oldImage, err := getBoundImageTx(ctx, tx, creative.ID)
	if err != nil {
		return models.Creative{}, nil, err
	}
	var detached *models.CreativeImage

	if imageChange.Set {
		sameImage := oldImage != nil && imageChange.Value != nil && oldImage.ID == strings.TrimSpace(*imageChange.Value)
		if !sameImage {
			if oldImage != nil {
				if _, err := tx.ExecContext(ctx, `
					UPDATE creative_images SET creative_id=NULL, updated_at=NOW() WHERE id=$1 AND creative_id=$2
				`, oldImage.ID, creative.ID); err != nil {
					return models.Creative{}, nil, err
				}
				detached = oldImage
			}
			if imageChange.Value != nil {
				if err := bindImageTx(ctx, tx, userID, creative.CampaignID, creative.ID, strings.TrimSpace(*imageChange.Value)); err != nil {
					return models.Creative{}, nil, err
				}
			}
		}
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE creatives SET creative_name=$3, adm=$4, banner_type=$5, trackers_macros=$6,
			w=$7, h=$8, title=$9, description=$10, updated_at=NOW()
		WHERE id=$2 AND EXISTS (
			SELECT 1 FROM campaigns c WHERE c.campaign_id=creatives.campaign_id AND c.user_id=$1
		)
		RETURNING id
	`, userID, creative.ID, creative.CreativeName, creative.ADM, creative.BannerType,
		jsonArg(creative.TrackersMacros), creative.W, creative.H, creative.Title, creative.Description)
	if err := row.Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Creative{}, nil, httpx.NotFound("creative not found")
		}
		return models.Creative{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return models.Creative{}, nil, err
	}
	updated, err := r.GetByIDOnly(ctx, creative.ID)
	return updated, detached, err
}

func (r *Repository) Delete(ctx context.Context, userID, creativeID string) (models.Creative, *models.CreativeImage, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Creative{}, nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, baseCreativeSelect+`
		WHERE cr.id=$2 AND c.user_id=$1
		FOR UPDATE OF cr
	`, userID, creativeID)
	creative, err := scanCreative(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Creative{}, nil, httpx.NotFound("creative not found")
	}
	if err != nil {
		return models.Creative{}, nil, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM creatives WHERE id=$1`, creativeID); err != nil {
		return models.Creative{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return models.Creative{}, nil, err
	}
	return creative, imageFromCreative(creative), nil
}

func (r *Repository) CreateImage(ctx context.Context, image models.CreativeImage) (models.CreativeImage, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO creative_images (
			id, user_id, campaign_id, s3_key, web_url, original_name, mime_type, file_format, size_bytes
		)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9
		FROM campaigns c
		WHERE c.campaign_id=$3 AND c.user_id=$2
		RETURNING id, user_id, campaign_id, creative_id, s3_key, web_url, original_name,
			mime_type, file_format, size_bytes, created_at, updated_at
	`, image.ID, image.UserID, image.CampaignID, image.S3Key, image.WebURL, image.OriginalName,
		image.MimeType, image.FileFormat, image.SizeBytes)
	created, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.CreativeImage{}, httpx.NotFound("campaign not found")
	}
	return created, err
}

func (r *Repository) ListImagesByCampaign(ctx context.Context, userID, campaignID string) ([]models.CreativeImage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ci.id, ci.user_id, ci.campaign_id, ci.creative_id, ci.s3_key, ci.web_url,
			ci.original_name, ci.mime_type, ci.file_format, ci.size_bytes, ci.created_at, ci.updated_at
		FROM creative_images ci
		JOIN campaigns c ON c.campaign_id=ci.campaign_id
		WHERE ci.campaign_id=$2 AND c.user_id=$1
	`, userID, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []models.CreativeImage
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (r *Repository) GetImage(ctx context.Context, imageID string) (models.CreativeImage, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, campaign_id, creative_id, s3_key, web_url, original_name,
			mime_type, file_format, size_bytes, created_at, updated_at
		FROM creative_images
		WHERE id=$1
	`, imageID)
	image, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.CreativeImage{}, httpx.NotFound("image not found")
	}
	return image, err
}

func (r *Repository) DeleteImageRecord(ctx context.Context, imageID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM creative_images WHERE id=$1 AND creative_id IS NULL`, imageID)
	if err != nil {
		return err
	}
	_, err = res.RowsAffected()
	return err
}

func bindImageTx(ctx context.Context, tx *sql.Tx, userID, campaignID, creativeID, imageID string) error {
	if imageID == "" {
		return httpx.BadRequest("image_id cannot be empty")
	}
	var boundID string
	err := tx.QueryRowContext(ctx, `
		UPDATE creative_images
		SET creative_id=$4, updated_at=NOW()
		WHERE id=$1 AND user_id=$2 AND campaign_id=$3 AND creative_id IS NULL
		RETURNING id
	`, imageID, userID, campaignID, creativeID).Scan(&boundID)
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.Conflict("image not found, belongs to another campaign, or is already used")
	}
	return err
}

func getBoundImageTx(ctx context.Context, tx *sql.Tx, creativeID string) (*models.CreativeImage, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, user_id, campaign_id, creative_id, s3_key, web_url, original_name,
			mime_type, file_format, size_bytes, created_at, updated_at
		FROM creative_images
		WHERE creative_id=$1
		FOR UPDATE
	`, creativeID)
	image, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &image, nil
}

const baseCreativeSelect = `
SELECT cr.id, cr.campaign_id, cr.creative_name, cr.adm, cr.banner_type, cr.trackers_macros,
	cr.w, cr.h, cr.title, cr.description,
	ci.id, ci.web_url, ci.original_name, ci.s3_key, ci.mime_type, ci.file_format,
	c.format_type
FROM creatives cr
JOIN campaigns c ON c.campaign_id=cr.campaign_id
LEFT JOIN creative_images ci ON ci.creative_id=cr.id
`

type scanner interface{ Scan(dest ...any) error }

func scanCreative(s scanner) (models.Creative, error) {
	var creative models.Creative
	var raw []byte
	var bannerType, title, description sql.NullString
	var imageID, imageURL, imageName, s3Key, imageMimeType, imageFormat sql.NullString
	var w, h sql.NullInt64
	err := s.Scan(&creative.ID, &creative.CampaignID, &creative.CreativeName, &creative.ADM, &bannerType,
		&raw, &w, &h, &title, &description, &imageID, &imageURL, &imageName, &s3Key,
		&imageMimeType, &imageFormat, &creative.FormatType)
	if err != nil {
		return models.Creative{}, err
	}
	creative.TrackersMacros, err = db.UnmarshalMacroMap(raw)
	if err != nil {
		return models.Creative{}, err
	}
	if bannerType.Valid {
		creative.BannerType = &bannerType.String
	}
	if w.Valid {
		value := int(w.Int64)
		creative.W = &value
	}
	if h.Valid {
		value := int(h.Int64)
		creative.H = &value
	}
	if title.Valid {
		creative.Title = &title.String
	}
	if description.Valid {
		creative.Description = &description.String
	}
	if imageID.Valid {
		creative.ImageID = &imageID.String
	}
	if imageURL.Valid {
		creative.ImageURL = &imageURL.String
	}
	if imageName.Valid {
		creative.ImageName = &imageName.String
	}
	if s3Key.Valid {
		creative.S3Key = &s3Key.String
	}
	if imageMimeType.Valid {
		creative.ImageMimeType = &imageMimeType.String
	}
	if imageFormat.Valid {
		creative.ImageFormat = &imageFormat.String
	}
	return creative, nil
}

func scanImage(s scanner) (models.CreativeImage, error) {
	var image models.CreativeImage
	var campaignID, creativeID sql.NullString
	err := s.Scan(&image.ID, &image.UserID, &campaignID, &creativeID, &image.S3Key,
		&image.WebURL, &image.OriginalName, &image.MimeType, &image.FileFormat, &image.SizeBytes,
		&image.CreatedAt, &image.UpdatedAt)
	if err != nil {
		return models.CreativeImage{}, err
	}
	if campaignID.Valid {
		image.CampaignID = campaignID.String
	}
	if creativeID.Valid {
		image.CreativeID = &creativeID.String
	}
	return image, nil
}

func imageFromCreative(creative models.Creative) *models.CreativeImage {
	if creative.ImageID == nil || creative.S3Key == nil {
		return nil
	}
	image := &models.CreativeImage{ID: *creative.ImageID, CampaignID: creative.CampaignID, S3Key: *creative.S3Key}
	image.CreativeID = &creative.ID
	if creative.ImageURL != nil {
		image.WebURL = *creative.ImageURL
	}
	if creative.ImageName != nil {
		image.OriginalName = *creative.ImageName
	}
	if creative.ImageMimeType != nil {
		image.MimeType = *creative.ImageMimeType
	}
	if creative.ImageFormat != nil {
		image.FileFormat = *creative.ImageFormat
	}
	return image
}

func jsonArg(value any) any {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
