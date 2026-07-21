package stats

import (
	"fmt"
	"strings"

	"twinbid-backend/internal/httpx"
)

var trafficFormatValues = map[string]string{
	"banner":   "BAN",
	"native":   "NAT",
	"popunder": "POP",
	"push":     "IPP",
}

var trafficTypeValues = map[string][]string{
	"mainstream": {"MAINSTREAM"},
	"adult":      {"ADULT"},
	// MIXED means that the campaign accepts both request traffic types.
	"mixed": {"MAINSTREAM", "ADULT"},
}

// The frontend deliberately sends human-readable browser groups. The raw ORTB
// table stores concrete browser names, so known groups must be expanded before
// building the ClickHouse predicate. Unknown values are preserved as-is.
var browserGroupValues = map[string][]string{
	"Chrome": {
		"Chrome", "Chromium", "Ubuntu Chromium", "Raspbian Chromium",
		"Kiwi Chrome", "Iron", "Comodo_Dragon", "JSChromeBrowser",
	},
	"Safari":          {"Safari"},
	"Edge":            {"Edge"},
	"Samsung Browser": {"Samsung Browser"},
	"Firefox":         {"Firefox"},
	"Opera":           {"Opera", "Opera Touch", "Opera Mini"},
	"Yandex Browser": {
		"YaBrowser", "YaApp_Android", "YandexSearch", "YaSearchBrowser", "YaSearchApp",
	},
	"Huawei / Honor Browser": {"Huawei Browser", "HonorBrowser"},
	"Miui Browser":           {"Miui Browser"},
	"HeyTap Browser":         {"HeyTapBrowser"},
	"Android Browser":        {"Android browser"},
	"Internet Explorer / Trident": {
		"Internet Explorer", "Trident",
	},
	"UC Browser": {"UCBrowser", "UCMobile", "UCPC", "UCTurbo", "UBrowser"},
	"QQ / Tencent Browser": {
		"QQBrowser", "MQQBrowser", "Mobile MQQBrowser", "QQ", "Qzone", "QZONEJSSDK",
	},
	"Quark":      {"Quark", "QuarkPC"},
	"DuckDuckGo": {"DuckDuckGo", "Mobile DuckDuckGo"},
	"Brave":      {"Brave"},
	"Vivaldi":    {"Vivaldi"},
	"Yahoo / YJApp": {
		"YJApp-IOS jp.co.yahoo.ipn.appli",
		"YJApp-ANDROID jp.co.yahoo.android.yjtop",
		"YJApp-IOS jp.co.yahoo.yjtrend01",
		"YJApp-ANDROID jp.co.yahoo.android.ybrowser",
		"YahooSearch", "YnoteiOS",
	},
	"Facebook App":       {"Facebook App"},
	"Instagram App":      {"Instagram App"},
	"TikTok / ByteDance": {"TikTok App", "bytedancewebview", "TTWebView"},
	"Twitter / X App":    {"Twitter for iPhone"},
	"WeChat / WeCom":     {"MicroMessenger", "wxwork"},
	"LINE App":           {"Safari Line", "Line"},
	"Snapchat":           {"Snapchat"},
	"Pinterest":          {"Pinterest"},
	"DingTalk":           {"DingTalk"},
	"KakaoTalk":          {"KAKAOTALK"},
	"Zalo":               {"Zalo iOS", "Zalo android"},
	"Douban":             {"com.douban.frodo"},
	"Baidu": {
		"swan", "tieba", "haokan", "ZhihuHybrid DefaultBrowser com.zhihu.android",
	},
	"Privacy Browsers": {"Avast", "AVG", "Norton", "Avira", "CCleaner", "MacKeeper", "ADG", "Phantom", "Blue Proxy"},
	"Smart TV": {
		"SmartTV", "Smart TV Build", "TV Bro", "TSBNetTV", "TeslaBrowser",
		"Tesla", "WebOS", "inext TV", "Changhong Andr0id TV Build",
	},
}

type trafficFilter struct {
	column    string
	values    []string
	mode      FilterMode
	modeField string
	normalize func(string) string
	expand    map[string][]string
}

func buildCalculatorPlan(req TrafficSegmentRequest, table string) (sqlPlan, error) {
	where, args, table, err := buildTrafficQueryParts(req, table)
	if err != nil {
		return sqlPlan{}, err
	}

	return sqlPlan{
		SQL: fmt.Sprintf(`
SELECT
    toUInt64(ifNull(sum(requests), 0)) AS potential_impressions
FROM %s
WHERE event_hour >= toStartOfDay(now('UTC')) - INTERVAL 1 DAY
  AND event_hour < toStartOfDay(now('UTC'))
  AND %s`, table, where),
		Args: args,
	}, nil
}

func buildRecommendBidPlan(req TrafficSegmentRequest, table string) (sqlPlan, error) {
	where, args, table, err := buildTrafficQueryParts(req, table)
	if err != nil {
		return sqlPlan{}, err
	}

	return sqlPlan{
		SQL: fmt.Sprintf(`
SELECT
    round(
        if(
            ifNull(sum(nonzero_win_dsp_price_count), 0) = 0,
            0,
            ifNull(sum(nonzero_win_dsp_price_sum), 0)
                / sum(nonzero_win_dsp_price_count)
        ),
        8
    ) AS average_bid
FROM %s
WHERE event_hour >= toStartOfDay(now('UTC')) - INTERVAL 1 DAY
  AND event_hour < toStartOfDay(now('UTC'))
  AND %s`, table, where),
		Args: args,
	}, nil
}

func buildTrafficQueryParts(req TrafficSegmentRequest, table string) (string, []any, string, error) {
	table, err := normalizeTable(table)
	if err != nil {
		return "", nil, "", err
	}

	formatValue, ok := trafficFormatValues[strings.ToLower(strings.TrimSpace(req.FormatType))]
	if !ok {
		return "", nil, "", httpx.BadRequest("invalid format_type")
	}

	trafficValues, ok := trafficTypeValues[strings.ToLower(strings.TrimSpace(req.TrafficType))]
	if !ok {
		return "", nil, "", httpx.BadRequest("invalid traffic_type")
	}

	parts := []string{
		"format = ?",
		"typic IN (" + valuePlaceholders(len(trafficValues)) + ")",
	}
	args := []any{formatValue}
	for _, value := range trafficValues {
		args = append(args, value)
	}

	filters := []trafficFilter{
		{
			column:    "geo",
			values:    req.Country,
			mode:      req.CountryMode,
			modeField: "country_mode",
			normalize: strings.ToUpper,
		},
		{
			column:    "lang",
			values:    req.Language,
			mode:      req.LanguageMode,
			modeField: "language_mode",
			normalize: strings.ToLower,
		},
		{
			column:    "device",
			values:    req.DeviceType,
			mode:      req.DeviceTypeMode,
			modeField: "device_type_mode",
			normalize: strings.ToLower,
		},
		{
			column:    "os",
			values:    req.OS,
			mode:      req.OSMode,
			modeField: "os_mode",
		},
		{
			column:    "browser",
			values:    req.Browser,
			mode:      req.BrowserMode,
			modeField: "browser_mode",
			expand:    browserGroupValues,
		},
		{
			column:    "site_id",
			values:    req.SiteID,
			mode:      req.SiteIDMode,
			modeField: "site_id_mode",
		},
	}

	for _, filter := range filters {
		values := normalizeTrafficFilterValues(filter.values, filter.normalize, filter.expand)
		if len(values) == 0 {
			continue
		}

		mode := filter.mode
		if mode == "" {
			mode = FilterModeInclude
		}
		if mode != FilterModeInclude && mode != FilterModeExclude {
			return "", nil, "", httpx.BadRequest("invalid " + filter.modeField)
		}

		operator := "IN"
		if mode == FilterModeExclude {
			operator = "NOT IN"
		}

		parts = append(parts, fmt.Sprintf(
			"%s %s (%s)",
			filter.column,
			operator,
			valuePlaceholders(len(values)),
		))
		for _, value := range values {
			args = append(args, value)
		}
	}

	return strings.Join(parts, " AND "), args, table, nil
}

func normalizeTrafficFilterValues(
	values []string,
	normalize func(string) string,
	expand map[string][]string,
) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))

	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if normalize != nil {
			value = normalize(value)
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if expanded, ok := expand[value]; ok {
			for _, raw := range expanded {
				appendValue(raw)
			}
			continue
		}
		appendValue(value)
	}

	return out
}
