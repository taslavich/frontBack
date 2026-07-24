package campaigns

import (
	"sort"
	"strings"

	"twinbid-backend/internal/models"
)

func applyPatchRequest(current *models.Campaign, req PatchCampaignRequest) {
	if req.CampaignName != nil {
		current.CampaignName = *req.CampaignName
	}
	if req.FormatType != nil {
		current.FormatType = *req.FormatType
	}
	if req.QualityType != nil {
		current.QualityType = *req.QualityType
	}
	if req.BrandNameSet {
		current.BrandName = req.BrandName
	}
	if req.HSet {
		current.H = req.H
	}
	if req.WSet {
		current.W = req.W
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if req.TrafficType != nil {
		current.TrafficType = *req.TrafficType
	}
	if req.Vertical != nil {
		current.Vertical = cloneTargetingMap(*req.Vertical)
	}
	if req.PricingModel != nil {
		current.PricingModel = *req.PricingModel
	}
	if req.BasePrice != nil {
		current.BasePrice = *req.BasePrice
	}
	if req.EvennessBySlotMode != nil {
		current.EvennessBySlotMode = *req.EvennessBySlotMode
	}
	if req.GoalTotalDollars != nil {
		current.GoalTotalDollars = *req.GoalTotalDollars
		if current.GoalTotalDollars > current.CumDoneDollars {
			current.NoBudgetNotified = false
		}
	}
	if req.CumDoneDollars != nil {
		current.CumDoneDollars = *req.CumDoneDollars
	}
	if req.StartTS != nil {
		current.StartTS = req.StartTS.UTC()
	}
	if req.EndTS != nil {
		current.EndTS = req.EndTS.UTC()
	}
	if req.ActiveIntervals != nil {
		current.ActiveIntervals = append([]models.ScheduleInterval(nil), (*req.ActiveIntervals)...)
	}
	if req.Country != nil {
		current.Country = cloneFilterForStorage(*req.Country)
	}
	if req.Language != nil {
		current.Language = cloneFilterForStorage(*req.Language)
	}
	if req.DeviceType != nil {
		current.DeviceType = cloneFilterForStorage(*req.DeviceType)
	}
	if req.OS != nil {
		current.OS = cloneFilterForStorage(*req.OS)
	}
	if req.Browser != nil {
		current.Browser = cloneFilterForStorage(*req.Browser)
	}
	if req.SiteID != nil {
		current.SiteID = cloneFilterForStorage(*req.SiteID)
	}
	if req.IP != nil {
		current.IP = cloneFilterForStorage(*req.IP)
	}
	if req.NoBudgetNotified != nil {
		current.NoBudgetNotified = *req.NoBudgetNotified
	}
}

func requiresTrafficReset(oldCampaign, newCampaign models.Campaign) bool {
	oldStatus := strings.ToLower(strings.TrimSpace(oldCampaign.Status))
	newStatus := strings.ToLower(strings.TrimSpace(newCampaign.Status))
	if oldStatus != "active" && newStatus == "active" {
		return true
	}
	if newCampaign.BasePrice > oldCampaign.BasePrice {
		return true
	}
	if normalizedString(oldCampaign.PricingModel) != normalizedString(newCampaign.PricingModel) {
		return true
	}
	if filterExpands(oldCampaign.Country, newCampaign.Country) ||
		filterExpands(oldCampaign.Language, newCampaign.Language) ||
		filterExpands(oldCampaign.DeviceType, newCampaign.DeviceType) ||
		filterExpands(oldCampaign.OS, newCampaign.OS) ||
		filterExpands(oldCampaign.Browser, newCampaign.Browser) ||
		filterExpands(oldCampaign.SiteID, newCampaign.SiteID) ||
		filterExpands(oldCampaign.IP, newCampaign.IP) {
		return true
	}
	if targetingMapExpands(oldCampaign.Vertical, newCampaign.Vertical) {
		return true
	}
	if trafficTypeRequiresReset(oldCampaign.TrafficType, newCampaign.TrafficType) {
		return true
	}
	if normalizedString(oldCampaign.QualityType) != normalizedString(newCampaign.QualityType) {
		return true
	}
	if normalizedString(oldCampaign.FormatType) != normalizedString(newCampaign.FormatType) {
		return true
	}
	if normalizedString(newCampaign.FormatType) == "banner" &&
		(!equalOptionalInt(oldCampaign.W, newCampaign.W) || !equalOptionalInt(oldCampaign.H, newCampaign.H)) {
		return true
	}
	return oldCampaign.EvennessBySlotMode && !newCampaign.EvennessBySlotMode
}

func filterExpands(oldFilter, newFilter models.TargetingFilter) bool {
	oldFilter = normalizeFilter(oldFilter)
	newFilter = normalizeFilter(newFilter)
	oldSet := stringSet(oldFilter.Objects)
	newSet := stringSet(newFilter.Objects)

	if oldFilter.IsWhiteList && !newFilter.IsWhiteList {
		return true
	}
	if oldFilter.IsWhiteList && newFilter.IsWhiteList {
		return hasSetDifference(newSet, oldSet)
	}
	if !oldFilter.IsWhiteList && !newFilter.IsWhiteList {
		return hasSetDifference(oldSet, newSet)
	}
	// blacklist -> whitelist is normally a narrowing. It still resets when a value
	// explicitly blocked before becomes explicitly allowed now.
	for value := range newSet {
		if _, wasBlocked := oldSet[value]; wasBlocked {
			return true
		}
	}
	return false
}

func targetingMapExpands(oldMap, newMap models.TargetingMap) bool {
	for key, newValue := range normalizeTargetingMap(newMap) {
		oldValue, existed := normalizeTargetingMap(oldMap)[key]
		if newValue > 0 && (!existed || oldValue <= 0) {
			return true
		}
	}
	return false
}

func trafficTypeRequiresReset(oldValue, newValue string) bool {
	oldValue = normalizedString(oldValue)
	newValue = normalizedString(newValue)
	if oldValue == newValue {
		return false
	}
	if oldValue == "mixed" && (newValue == "mainstream" || newValue == "adult") {
		return false
	}
	return (oldValue == "mainstream" || oldValue == "adult" || oldValue == "mixed") &&
		(newValue == "mainstream" || newValue == "adult" || newValue == "mixed")
}

func cloneCampaignForComparison(c models.Campaign) models.Campaign {
	c.Vertical = cloneTargetingMap(c.Vertical)
	c.ActiveIntervals = append([]models.ScheduleInterval(nil), c.ActiveIntervals...)
	c.Country = cloneFilterForStorage(c.Country)
	c.Language = cloneFilterForStorage(c.Language)
	c.DeviceType = cloneFilterForStorage(c.DeviceType)
	c.OS = cloneFilterForStorage(c.OS)
	c.Browser = cloneFilterForStorage(c.Browser)
	c.SiteID = cloneFilterForStorage(c.SiteID)
	c.IP = cloneFilterForStorage(c.IP)
	return c
}

func cloneFilterForStorage(value models.TargetingFilter) models.TargetingFilter {
	objects := append([]string(nil), value.Objects...)
	if objects == nil {
		objects = []string{}
	}
	return models.TargetingFilter{IsWhiteList: value.IsWhiteList, Objects: objects}
}

func normalizeFilter(value models.TargetingFilter) models.TargetingFilter {
	set := stringSet(value.Objects)
	objects := make([]string, 0, len(set))
	for item := range set {
		objects = append(objects, item)
	}
	sort.Strings(objects)
	return models.TargetingFilter{IsWhiteList: value.IsWhiteList, Objects: objects}
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = normalizedString(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func hasSetDifference(left, right map[string]struct{}) bool {
	for value := range left {
		if _, ok := right[value]; !ok {
			return true
		}
	}
	return false
}

func normalizeTargetingMap(value models.TargetingMap) models.TargetingMap {
	out := make(models.TargetingMap, len(value))
	for key, flag := range value {
		key = normalizedString(key)
		if key != "" {
			out[key] = flag
		}
	}
	return out
}

func cloneTargetingMap(value models.TargetingMap) models.TargetingMap {
	out := make(models.TargetingMap, len(value))
	for key, flag := range value {
		out[key] = flag
	}
	return out
}

func normalizedString(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func equalOptionalInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
