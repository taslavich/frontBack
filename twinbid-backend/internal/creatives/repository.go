package creatives

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

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
		c, err := scanCreative(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, userID, creativeID string) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, baseCreativeSelect+`WHERE cr.id=$2 AND c.user_id=$1`, userID, creativeID)
	c, err := scanCreative(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Creative{}, httpx.NotFound("creative not found")
	}
	return c, err
}

func (r *Repository) Create(ctx context.Context, cr models.Creative) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO creatives (campaign_id, creative_name, link, trackers_macros, w, h, name, s3_file_path, file_format, title, description)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id
	`, cr.CampaignID, cr.CreativeName, cr.Link, jsonArg(cr.TrackersMacros), cr.W, cr.H, cr.Name, cr.S3FilePath, cr.FileFormat, cr.Title, cr.Description)
	if err := row.Scan(&cr.ID); err != nil {
		return models.Creative{}, err
	}
	return r.GetByIDOnly(ctx, cr.ID)
}

func (r *Repository) GetByIDOnly(ctx context.Context, creativeID string) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, baseCreativeSelect+`WHERE cr.id=$1`, creativeID)
	return scanCreative(row)
}

func (r *Repository) Update(ctx context.Context, userID string, cr models.Creative) (models.Creative, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE creatives SET creative_name=$3, link=$4, trackers_macros=$5, w=$6, h=$7, name=$8,
			s3_file_path=$9, file_format=$10, title=$11, description=$12, updated_at=NOW()
		WHERE id=$2 AND EXISTS (SELECT 1 FROM campaigns c WHERE c.campaign_id=creatives.campaign_id AND c.user_id=$1)
		RETURNING id
	`, userID, cr.ID, cr.CreativeName, cr.Link, jsonArg(cr.TrackersMacros), cr.W, cr.H, cr.Name, cr.S3FilePath, cr.FileFormat, cr.Title, cr.Description)
	var id string
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Creative{}, httpx.NotFound("creative not found")
		}
		return models.Creative{}, err
	}
	return r.GetByIDOnly(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, userID, creativeID string) (models.Creative, error) {
	cr, err := r.Get(ctx, userID, creativeID)
	if err != nil {
		return models.Creative{}, err
	}
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM creatives cr USING campaigns c
		WHERE cr.campaign_id=c.campaign_id AND c.user_id=$1 AND cr.id=$2
	`, userID, creativeID)
	if err != nil {
		return models.Creative{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return models.Creative{}, err
	}
	if n == 0 {
		return models.Creative{}, httpx.NotFound("creative not found")
	}
	return cr, nil
}

const baseCreativeSelect = `
SELECT cr.id, cr.campaign_id, cr.creative_name, cr.link, cr.trackers_macros, cr.w, cr.h, cr.name,
	cr.s3_file_path, cr.file_format, cr.title, cr.description, c.format_type
FROM creatives cr
JOIN campaigns c ON c.campaign_id = cr.campaign_id
`

type scanner interface{ Scan(dest ...any) error }

func scanCreative(s scanner) (models.Creative, error) {
	var cr models.Creative
	var raw []byte
	var w, h sql.NullInt64
	var name, s3Path, fileFormat, title, description sql.NullString
	err := s.Scan(&cr.ID, &cr.CampaignID, &cr.CreativeName, &cr.Link, &raw, &w, &h, &name, &s3Path, &fileFormat, &title, &description, &cr.FormatType)
	if err != nil {
		return models.Creative{}, err
	}
	cr.TrackersMacros, err = db.UnmarshalMacroMap(raw)
	if err != nil {
		return models.Creative{}, err
	}
	if w.Valid {
		v := int(w.Int64)
		cr.W = &v
	}
	if h.Valid {
		v := int(h.Int64)
		cr.H = &v
	}
	if name.Valid {
		cr.Name = &name.String
	}
	if s3Path.Valid {
		cr.S3FilePath = &s3Path.String
	}
	if fileFormat.Valid {
		cr.FileFormat = &fileFormat.String
	}
	if title.Valid {
		cr.Title = &title.String
	}
	if description.Valid {
		cr.Description = &description.String
	}
	return cr, nil
}

func jsonArg(v any) any { b, _ := json.Marshal(v); return string(b) }
