package campaigns

import (
	"fmt"
	"regexp"
	"strings"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

var osVersionTargetRE = regexp.MustCompile(`(?i)^(ios|android)\s+([0-9]+)(?:\.([0-9]+))?$`)

type parsedOSVersionTarget struct {
	os string
}

func parseOSVersionTarget(value string) (parsedOSVersionTarget, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "Android 12L") {
		return parsedOSVersionTarget{os: "android"}, nil
	}
	match := osVersionTargetRE.FindStringSubmatch(value)
	if match == nil {
		return parsedOSVersionTarget{}, fmt.Errorf("expected iOS/Android major or major.minor version")
	}
	return parsedOSVersionTarget{os: strings.ToLower(match[1])}, nil
}

func validateOSVersionTargeting(osFilter, versionFilter models.TargetingFilter) error {
	if len(versionFilter.Objects) == 0 {
		return nil
	}
	if !osFilter.IsWhiteList {
		return httpx.BadRequest("os_version requires os to be a whitelist containing iOS and/or Android")
	}

	selectedOS := make(map[string]bool, 2)
	for _, raw := range osFilter.Objects {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "ios", "iphone os", "ipad os", "iphoneos", "ipados", "iphone", "ipad":
			selectedOS["ios"] = true
		case "android", "android os", "androidos":
			selectedOS["android"] = true
		}
	}

	for _, raw := range versionFilter.Objects {
		target, err := parseOSVersionTarget(raw)
		if err != nil {
			return httpx.BadRequest(fmt.Sprintf("invalid os_version %q: %v", raw, err))
		}
		if !selectedOS[target.os] {
			return httpx.BadRequest(fmt.Sprintf("os_version %q requires %s in os whitelist", raw, displayOSName(target.os)))
		}
	}
	return nil
}

func displayOSName(osName string) string {
	if osName == "ios" {
		return "iOS"
	}
	return "Android"
}
