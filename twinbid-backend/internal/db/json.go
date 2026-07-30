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

func UnmarshalMacroMap(raw []byte) (models.MacroMap, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return models.MacroMap{}, nil
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("macro map json: %w", err)
	}

	out := make(models.MacroMap, len(values))
	for key, value := range values {
		var parameterName string
		if err := json.Unmarshal(value, &parameterName); err == nil {
			if parameterName != "" {
				out[key] = parameterName
			}
			continue
		}

		var enabled bool
		if err := json.Unmarshal(value, &enabled); err == nil {
			if enabled {
				out[key] = key
			}
			continue
		}

		var numeric int
		if err := json.Unmarshal(value, &numeric); err == nil {
			if numeric != 0 {
				out[key] = key
			}
			continue
		}

		return nil, fmt.Errorf("macro map key %q has unsupported value %s", key, string(value))
	}

	return out, nil
}

func UnmarshalTargetingFilter(raw []byte) (models.TargetingFilter, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return models.NormalizeTargetingFilter(models.TargetingFilter{}), nil
	}

	var out models.TargetingFilter
	if err := json.Unmarshal(raw, &out); err != nil {
		return models.TargetingFilter{}, fmt.Errorf("targeting filter json: %w", err)
	}

	return models.NormalizeTargetingFilter(out), nil
}

func UnmarshalTargeting(raw []byte) (models.TargetingMap, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
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
