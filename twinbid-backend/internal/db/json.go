package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"twinbid-backend/internal/models"
)

func JSONValue(v any) (driver.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func UnmarshalTargeting(raw []byte) (models.TargetingMap, error) {
	if len(raw) == 0 {
		return models.TargetingMap{}, nil
	}
	var out models.TargetingMap
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("targeting json: %w", err)
	}
	if out == nil {
		out = models.TargetingMap{}
	}
	return out, nil
}

func UnmarshalIntervals(raw []byte) ([]models.ScheduleInterval, error) {
	if len(raw) == 0 {
		return []models.ScheduleInterval{}, nil
	}
	var out []models.ScheduleInterval
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("active_intervals json: %w", err)
	}
	if out == nil {
		out = []models.ScheduleInterval{}
	}
	return out, nil
}
