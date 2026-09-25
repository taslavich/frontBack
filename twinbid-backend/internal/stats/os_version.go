package stats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"twinbid-backend/internal/httpx"
)

var osVersionFilterRE = regexp.MustCompile(`(?i)^(ios|android)\s+([0-9]+)(?:\.([0-9]+))?$`)

type osVersionFilterTarget struct {
	os         string
	components []uint64
	special    string
}

func parseOSVersionFilterTarget(raw string) (osVersionFilterTarget, error) {
	value := strings.TrimSpace(raw)
	if strings.EqualFold(value, "Android 12L") {
		return osVersionFilterTarget{os: "android", special: "12l"}, nil
	}
	match := osVersionFilterRE.FindStringSubmatch(value)
	if match == nil {
		return osVersionFilterTarget{}, httpx.BadRequest(fmt.Sprintf("invalid os_version %q", raw))
	}

	components := make([]uint64, 0, 2)
	for _, part := range match[2:] {
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return osVersionFilterTarget{}, httpx.BadRequest(fmt.Sprintf("invalid os_version %q", raw))
		}
		components = append(components, n)
	}
	return osVersionFilterTarget{os: strings.ToLower(match[1]), components: components}, nil
}

func osVersionOSPredicate(osName string) string {
	if osName == "ios" {
		return "lowerUTF8(os) IN ('ios','iphone os','ipad os','iphoneos','ipados','iphone','ipad')"
	}
	return "lowerUTF8(os) IN ('android','android os','androidos')"
}

// buildOSVersionPredicate applies the same component semantics as ADV. Every
// raw numeric component must parse as an integer, so 18.7 matches 18.7,
// 18.7.1 and 18.07.10, but never 18.70 or malformed values. Android 12L is a
// deliberately separate non-numeric branch.
func buildOSVersionPredicate(values []string) (string, []any, error) {
	predicates := make([]string, 0, len(values))
	args := make([]any, 0, len(values)*2)
	for _, raw := range values {
		target, err := parseOSVersionFilterTarget(raw)
		if err != nil {
			return "", nil, err
		}

		versionExpr := "replaceAll(lowerUTF8(os_version), '_', '.')"
		if target.special != "" {
			predicates = append(predicates, fmt.Sprintf("(%s AND %s = ?)", osVersionOSPredicate(target.os), versionExpr))
			args = append(args, target.special)
			continue
		}

		partsExpr := fmt.Sprintf("splitByChar('.', %s)", versionExpr)
		validNumericExpr := fmt.Sprintf("arrayAll(part -> isNotNull(toUInt64OrNull(part)), %s)", partsExpr)
		componentPredicates := make([]string, 0, len(target.components))
		for i, component := range target.components {
			componentPredicates = append(componentPredicates,
				fmt.Sprintf("toUInt64OrNull(arrayElement(%s, %d)) = ?", partsExpr, i+1))
			args = append(args, component)
		}
		versionPredicate := validNumericExpr
		if len(componentPredicates) > 0 {
			versionPredicate += " AND " + strings.Join(componentPredicates, " AND ")
		}
		predicates = append(predicates, fmt.Sprintf("(%s AND (%s))", osVersionOSPredicate(target.os), versionPredicate))
	}
	if len(predicates) == 0 {
		return "", nil, nil
	}
	return "(" + strings.Join(predicates, " OR ") + ")", args, nil
}

const osVersionGroupExpr = `concat(
    multiIf(
        lowerUTF8(os) IN ('ios','iphone os','ipad os','iphoneos','ipados','iphone','ipad'), 'iOS',
        lowerUTF8(os) IN ('android','android os','androidos'), 'Android',
        os
    ),
    '|',
    os_version
)`
