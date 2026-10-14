package publisher

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Keep legacy stored utc_3 values unchanged; normalize only publisher queries.
// Etc/GMT uses the inverse sign and is understood by both CH and Postgres.
func NormalizeTimezone(raw string) (string, error) {
	if raw == "" {
		return "UTC", nil
	}
	if strings.HasPrefix(raw, "utc_") {
		hours, e := strconv.Atoi(strings.TrimPrefix(raw, "utc_"))
		if e != nil || hours < -12 || hours > 14 {
			return "", fmt.Errorf("invalid legacy timezone %q", raw)
		}
		if hours == 0 {
			return "UTC", nil
		}
		raw = fmt.Sprintf("Etc/GMT%+d", -hours)
	}
	if _, e := time.LoadLocation(raw); e != nil {
		return "", e
	}
	return raw, nil
}
