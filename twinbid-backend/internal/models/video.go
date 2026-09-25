package models

import "strings"

const (
	VideoFormatInstream  = "instream"
	VideoFormatOutstream = "outstream"
	VideoFormatPopup     = "video_popup"
)

func NormalizeVideoFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case VideoFormatInstream:
		return VideoFormatInstream
	case VideoFormatOutstream, "outstream_standard", "outstream_slider":
		return VideoFormatOutstream
	case VideoFormatPopup:
		return VideoFormatPopup
	default:
		return ""
	}
}

// VideoCreativeMetadata contains technical information about a VIDEO creative.
// It is not advertiser targeting: ADV uses these fields only to reject
// technically incompatible imp.video requests.
type VideoCreativeMetadata struct {
	Mimes      []string `json:"mimes,omitempty"`
	Duration   int      `json:"duration,omitempty"`
	Protocols  []int    `json:"protocols,omitempty"`
	API        []int    `json:"api,omitempty"`
	Attributes []int    `json:"battr,omitempty"`
	Bitrate    int      `json:"bitrate,omitempty"`
	Linearity  int      `json:"linearity,omitempty"`
	Skippable  *bool    `json:"skippable,omitempty"`
}

func NormalizeVideoCreativeMetadata(v *VideoCreativeMetadata) *VideoCreativeMetadata {
	if v == nil {
		return nil
	}
	out := *v
	out.Mimes = normalizeStringSlice(v.Mimes, true)
	out.Protocols = normalizeIntSlice(v.Protocols)
	out.API = normalizeIntSlice(v.API)
	out.Attributes = normalizeIntSlice(v.Attributes)
	if v.Skippable != nil {
		value := *v.Skippable
		out.Skippable = &value
	}
	return &out
}

func normalizeStringSlice(values []string, lower bool) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func normalizeIntSlice(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if out == nil {
		out = []int{}
	}
	return out
}
