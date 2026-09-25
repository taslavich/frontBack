package creatives

import (
	"bytes"
	"encoding/json"
	"fmt"

	"twinbid-backend/internal/models"
)

type CreateCreativeRequest struct {
	CreativeName   string                        `json:"creative_name"`
	ADM            string                        `json:"adm"`
	BannerType     *string                       `json:"banner_type"`
	ImageID        *string                       `json:"image_id"`
	TrackersMacros models.MacroMap               `json:"trackers_macros"`
	W              *int                          `json:"w"`
	H              *int                          `json:"h"`
	Title          *string                       `json:"title"`
	Description    *string                       `json:"description"`
	VideoFormat    *string                       `json:"video_format"`
	VideoMetadata  *models.VideoCreativeMetadata `json:"video_metadata"`
}

type PatchCreativeRequest struct {
	CreativeName   *string                       `json:"creative_name"`
	ADM            *string                       `json:"adm"`
	BannerType     *string                       `json:"banner_type"`
	ImageID        OptionalString                `json:"image_id"`
	TrackersMacros *models.MacroMap              `json:"trackers_macros"`
	W              *int                          `json:"w"`
	H              *int                          `json:"h"`
	Title          *string                       `json:"title"`
	Description    *string                       `json:"description"`
	VideoFormat    OptionalString                `json:"video_format"`
	VideoMetadata  OptionalVideoCreativeMetadata `json:"video_metadata"`
}

// OptionalString distinguishes an omitted JSON field from an explicit null.
// It is used by PATCH so image_id:null can remove the current image.
type OptionalString struct {
	Set   bool
	Value *string
}

func (v *OptionalString) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("image_id must be a string or null: %w", err)
	}
	v.Value = &value
	return nil
}

// OptionalVideoCreativeMetadata distinguishes an omitted PATCH field from an explicit null.
type OptionalVideoCreativeMetadata struct {
	Set   bool
	Value *models.VideoCreativeMetadata
}

func (v *OptionalVideoCreativeMetadata) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var value models.VideoCreativeMetadata
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("video_metadata must be an object or null: %w", err)
	}
	v.Value = &value
	return nil
}
