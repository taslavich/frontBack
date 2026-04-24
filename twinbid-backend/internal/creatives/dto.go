package creatives

import "twinbid-backend/internal/models"

type FormCreativeRequest struct {
	CreativeName   string              `json:"creative_name"`
	Link           string              `json:"link"`
	TrackersMacros models.TargetingMap `json:"trackers_macros"`
	W              *int                `json:"w"`
	H              *int                `json:"h"`
	Title          *string             `json:"title"`
	Description    *string             `json:"description"`
}
