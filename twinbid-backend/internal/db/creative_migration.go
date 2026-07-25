package db

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"strings"
)

type legacyBannerADMRow struct {
	creativeID string
	oldADM     string
	mediaURL   string
	mimeType   string
	width      int
	height     int
}

// migrateLegacyBannerADM converts only legacy banner rows that still contain a
// plain click URL in creatives.adm. New creatives do not populate
// creatives.s3_file_path, so this migration cannot rewrite newly-created ADM.
func migrateLegacyBannerADM(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT cr.id::text, cr.adm, ci.web_url, ci.mime_type,
		       COALESCE(cr.w, 0), COALESCE(cr.h, 0)
		FROM creatives cr
		JOIN campaigns c ON c.campaign_id=cr.campaign_id
		JOIN creative_images ci ON ci.creative_id=cr.id
		WHERE c.format_type='banner'
		  AND cr.banner_type='img'
		  AND cr.s3_file_path IS NOT NULL
		  AND cr.s3_file_path<>''
		  AND ltrim(cr.adm) NOT LIKE '<%'
	`)
	if err != nil {
		return fmt.Errorf("query legacy banner ADM rows: %w", err)
	}

	legacyRows := make([]legacyBannerADMRow, 0)
	for rows.Next() {
		var row legacyBannerADMRow
		if err := rows.Scan(&row.creativeID, &row.oldADM, &row.mediaURL, &row.mimeType, &row.width, &row.height); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy banner ADM row: %w", err)
		}
		legacyRows = append(legacyRows, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate legacy banner ADM rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy banner ADM rows: %w", err)
	}

	for _, row := range legacyRows {
		adm := buildLegacyBannerADM(row.oldADM, row.mediaURL, row.mimeType, row.width, row.height)
		if adm == "" {
			continue
		}
		result, err := db.ExecContext(ctx, `
			UPDATE creatives
			SET adm=$2, updated_at=NOW()
			WHERE id=$1::uuid AND adm=$3 AND banner_type='img'
		`, row.creativeID, adm, row.oldADM)
		if err != nil {
			return fmt.Errorf("update legacy banner ADM %s: %w", row.creativeID, err)
		}
		if _, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("read legacy banner ADM update result %s: %w", row.creativeID, err)
		}
	}
	return nil
}

func buildLegacyBannerADM(targetURL, mediaURL, mimeType string, width, height int) string {
	targetURL = strings.TrimSpace(targetURL)
	mediaURL = strings.TrimSpace(mediaURL)
	if targetURL == "" || mediaURL == "" {
		return ""
	}

	target := html.EscapeString(targetURL)
	media := html.EscapeString(mediaURL)
	dimensions := ""
	if width > 0 && height > 0 {
		dimensions = fmt.Sprintf(` width="%d" height="%d"`, width, height)
	}

	var visual string
	if strings.EqualFold(strings.TrimSpace(mimeType), "video/mp4") {
		visual = fmt.Sprintf(`<video src="%s"%s autoplay muted loop playsinline style="display:block;border:0"></video>`, media, dimensions)
	} else {
		visual = fmt.Sprintf(`<img src="%s"%s alt="" style="display:block;border:0">`, media, dimensions)
	}
	return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, target, visual)
}
