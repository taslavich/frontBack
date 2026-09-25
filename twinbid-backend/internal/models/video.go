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

// VideoCreativeMetadata is server-derived technical metadata for an uploaded
// VIDEO creative. Business targeting remains limited to VideoFormat; these
// fields are used only to prove OpenRTB technical compatibility.
type VideoCreativeMetadata struct {
	Mimes      []string `json:"mimes,omitempty"`
	Duration   int      `json:"duration,omitempty"`
	Protocols  []int    `json:"protocols,omitempty"`
	API        []int    `json:"api,omitempty"`
	Attributes []int    `json:"battr,omitempty"`
	Bitrate    int      `json:"bitrate,omitempty"`
	Linearity  int      `json:"linearity,omitempty"`
	Skippable  *bool    `json:"skippable,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	Codec      string   `json:"codec,omitempty"`
	FileSize   int64    `json:"file_size,omitempty"`
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
	out.Codec = strings.ToLower(strings.TrimSpace(v.Codec))
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
