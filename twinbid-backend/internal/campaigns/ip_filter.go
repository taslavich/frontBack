package campaigns

import (
	"fmt"
	"net/netip"
	"strings"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

// normalizeAndValidateIPv4TargetingFilter validates every configured value in
// the existing campaign IP targeting filter. Values may be either an exact
// IPv4 address or an IPv4 CIDR prefix. CIDRs are stored in canonical network
// form (for example 192.168.1.123/24 becomes 192.168.1.0/24).
//
// The filter is mutated only when every value is valid, so callers can safely
// return an HTTP 400 without persisting a partially normalized filter.
func normalizeAndValidateIPv4TargetingFilter(value *models.TargetingFilter) error {
	if value == nil {
		return nil
	}
	if value.Objects == nil {
		value.Objects = []string{}
		return nil
	}

	normalized := make([]string, 0, len(value.Objects))
	seen := make(map[string]struct{}, len(value.Objects))
	problems := make([]string, 0)

	for index, rawValue := range value.Objects {
		raw := strings.TrimSpace(rawValue)
		if raw == "" {
			problems = append(problems, fmt.Sprintf("ip.objects[%d]=%q is empty", index, rawValue))
			continue
		}

		canonical, err := normalizeIPv4OrCIDR(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("ip.objects[%d]=%q: %v", index, rawValue, err))
			continue
		}
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}

	if len(problems) > 0 {
		return httpx.BadRequest("invalid IP targeting values: " + strings.Join(problems, "; "))
	}

	value.Objects = normalized
	return nil
}

func normalizeIPv4OrCIDR(raw string) (string, error) {
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return "", fmt.Errorf("invalid IPv4 CIDR")
		}
		if !prefix.Addr().Is4() {
			return "", fmt.Errorf("IPv6 is not allowed in the IPv4 IP filter")
		}
		return prefix.Masked().String(), nil
	}

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return "", fmt.Errorf("invalid IPv4 address")
	}
	if !addr.Is4() {
		return "", fmt.Errorf("IPv6 is not allowed in the IPv4 IP filter")
	}
	return addr.String(), nil
}
