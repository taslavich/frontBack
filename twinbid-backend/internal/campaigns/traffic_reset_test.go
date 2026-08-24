package campaigns

import (
	"math"
	"testing"
	"time"

	"twinbid-backend/internal/models"
)

func baseResetCampaign() models.Campaign {
	w, h := 300, 250
	return models.Campaign{
		Status: "active", FormatType: "banner", PricingModel: "cpm", BasePrice: 1,
		TrafficType: "mainstream", QualityType: "usual", EvennessBySlotMode: true,
		W: &w, H: &h, GoalTotalDollars: 100,
		Vertical: models.TargetingMap{"games": 1},
		Country:  models.TargetingFilter{IsWhiteList: true, Objects: []string{"DE"}},
	}
}

func TestRequiresTrafficReset(t *testing.T) {
	tests := []struct {
		name string
		old  func(*models.Campaign)
		edit func(*models.Campaign)
		want bool
	}{
		{name: "same state", want: false},
		{
			name: "inactive to active",
			old:  func(c *models.Campaign) { c.Status = "paused" },
			edit: func(c *models.Campaign) { c.Status = "active" },
			want: true,
		},
		{name: "active to inactive", edit: func(c *models.Campaign) { c.Status = "paused" }, want: false},
		{name: "price increase", edit: func(c *models.Campaign) { c.BasePrice = 2 }, want: true},
		{name: "price decrease", edit: func(c *models.Campaign) { c.BasePrice = .5 }, want: false},
		{name: "pricing model", edit: func(c *models.Campaign) { c.PricingModel = "cpc" }, want: true},
		{name: "whitelist expands", edit: func(c *models.Campaign) { c.Country.Objects = []string{"DE", "FR"} }, want: true},
		{name: "whitelist narrows", edit: func(c *models.Campaign) { c.Country.Objects = nil }, want: false},
		{name: "whitelist order and duplicates", edit: func(c *models.Campaign) { c.Country.Objects = []string{"de", "DE"} }, want: false},
		{name: "whitelist to blacklist", edit: func(c *models.Campaign) { c.Country.IsWhiteList = false }, want: true},
		{
			name: "blacklist removal",
			old: func(c *models.Campaign) {
				c.Country = models.TargetingFilter{IsWhiteList: false, Objects: []string{"DE", "FR"}}
			},
			edit: func(c *models.Campaign) {
				c.Country = models.TargetingFilter{IsWhiteList: false, Objects: []string{"DE"}}
			},
			want: true,
		},
		{
			name: "blacklist addition",
			old: func(c *models.Campaign) {
				c.Country = models.TargetingFilter{IsWhiteList: false, Objects: []string{"DE"}}
			},
			edit: func(c *models.Campaign) {
				c.Country = models.TargetingFilter{IsWhiteList: false, Objects: []string{"DE", "FR"}}
			},
			want: false,
		},
		{name: "vertical expands", edit: func(c *models.Campaign) { c.Vertical["finance"] = 1 }, want: true},
		{name: "vertical narrows", edit: func(c *models.Campaign) { delete(c.Vertical, "games") }, want: false},
		{name: "mainstream to mixed", edit: func(c *models.Campaign) { c.TrafficType = "mixed" }, want: true},
		{name: "mainstream to adult", edit: func(c *models.Campaign) { c.TrafficType = "adult" }, want: true},
		{
			name: "mixed to mainstream",
			old:  func(c *models.Campaign) { c.TrafficType = "mixed" },
			edit: func(c *models.Campaign) { c.TrafficType = "mainstream" },
			want: false,
		},
		{name: "quality changes", edit: func(c *models.Campaign) { c.QualityType = "high" }, want: true},
		{name: "format changes", edit: func(c *models.Campaign) { c.FormatType = "native" }, want: true},
		{name: "banner size changes", edit: func(c *models.Campaign) { v := 728; c.W = &v }, want: true},
		{
			name: "non-banner size changes",
			old:  func(c *models.Campaign) { c.FormatType = "native" },
			edit: func(c *models.Campaign) { v := 728; c.W = &v },
			want: false,
		},
		{name: "disable VPN blocking expands traffic", old: func(c *models.Campaign) { c.BlockVPN = true }, edit: func(c *models.Campaign) { c.BlockVPN = false }, want: true},
		{name: "enable VPN blocking narrows traffic", edit: func(c *models.Campaign) { c.BlockVPN = true }, want: false},
		{name: "disable evenness", edit: func(c *models.Campaign) { c.EvennessBySlotMode = false }, want: true},
		{
			name: "enable evenness",
			old:  func(c *models.Campaign) { c.EvennessBySlotMode = false },
			edit: func(c *models.Campaign) { c.EvennessBySlotMode = true },
			want: false,
		},
		{name: "goal change", edit: func(c *models.Campaign) { c.GoalTotalDollars = 200 }, want: false},
		{name: "schedule change", edit: func(c *models.Campaign) { c.StartTS = time.Now().UTC() }, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldCampaign := cloneCampaignForComparison(baseResetCampaign())
			if tt.old != nil {
				tt.old(&oldCampaign)
			}
			newCampaign := cloneCampaignForComparison(oldCampaign)
			if tt.edit != nil {
				tt.edit(&newCampaign)
			}
			if got := requiresTrafficReset(oldCampaign, newCampaign); got != tt.want {
				t.Fatalf("requiresTrafficReset()=%v want %v\nold=%+v\nnew=%+v", got, tt.want, oldCampaign, newCampaign)
			}
		})
	}
}

func TestCampaignChargePrice(t *testing.T) {
	tests := []struct {
		name        string
		formatType  string
		pricing     string
		basePrice   float64
		wantPrice   float64
		wantDefined bool
	}{
		{name: "cpm banner", formatType: "banner", pricing: "cpm", basePrice: 1.5, wantPrice: 0.0015, wantDefined: true},
		{name: "cpm popunder", formatType: "popunder", pricing: "cpm", basePrice: 1.5, wantPrice: 0.0015, wantDefined: true},
		{name: "cpc popunder keeps legacy cpm storage", formatType: "popunder", pricing: "cpc", basePrice: 1.5, wantPrice: 0.0015, wantDefined: true},
		{name: "cpc non popunder", formatType: "push", pricing: "cpc", basePrice: 0.25, wantPrice: 0.25, wantDefined: true},
		{name: "normalizes values", formatType: " PopUnder ", pricing: " CPC ", basePrice: 2, wantPrice: 0.002, wantDefined: true},
		{name: "zero base price", formatType: "popunder", pricing: "cpc", basePrice: 0, wantDefined: false},
		{name: "negative base price", formatType: "popunder", pricing: "cpc", basePrice: -1, wantDefined: false},
		{name: "unknown pricing", formatType: "banner", pricing: "cpa", basePrice: 1, wantDefined: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := campaignChargePrice(models.Campaign{
				FormatType:   tt.formatType,
				PricingModel: tt.pricing,
				BasePrice:    tt.basePrice,
			})
			if ok != tt.wantDefined {
				t.Fatalf("campaignChargePrice() defined=%v want %v", ok, tt.wantDefined)
			}
			if ok && math.Abs(got-tt.wantPrice) > 1e-12 {
				t.Fatalf("campaignChargePrice()=%v want %v", got, tt.wantPrice)
			}
		})
	}
}

func TestApplyPatchRequestNoBudgetReset(t *testing.T) {
	floatPtr := func(v float64) *float64 { return &v }
	stringPtr := func(v string) *string { return &v }
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name    string
		current models.Campaign
		patch   PatchCampaignRequest
		want    bool
	}{
		{
			name: "goal increase still insufficient keeps notification",
			current: models.Campaign{
				FormatType: "popunder", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5, CumDoneDollars: 4.9995, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{GoalTotalDollars: floatPtr(5.0005)},
			want:  true,
		},
		{
			name: "goal increase enough for next pop event resets notification",
			current: models.Campaign{
				FormatType: "popunder", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5, CumDoneDollars: 4.9995, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{GoalTotalDollars: floatPtr(5.002)},
			want:  false,
		},
		{
			name: "base price decrease can restore budget",
			current: models.Campaign{
				FormatType: "popunder", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5, CumDoneDollars: 4.9995, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{BasePrice: floatPtr(0.4)},
			want:  false,
		},
		{
			name: "pricing model change uses resulting charge",
			current: models.Campaign{
				FormatType: "banner", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5.001, CumDoneDollars: 4.999, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{PricingModel: stringPtr("cpm")},
			want:  false,
		},
		{
			name: "format change uses resulting charge",
			current: models.Campaign{
				FormatType: "push", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5.001, CumDoneDollars: 4.999, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{FormatType: stringPtr("popunder")},
			want:  false,
		},
		{
			name: "unrelated patch does not reset notification",
			current: models.Campaign{
				FormatType: "popunder", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 10, CumDoneDollars: 1, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{CampaignName: stringPtr("renamed")},
			want:  true,
		},
		{
			name: "explicit notification value overrides automatic reset",
			current: models.Campaign{
				FormatType: "popunder", PricingModel: "cpc", BasePrice: 1.5,
				GoalTotalDollars: 5, CumDoneDollars: 4.9995, NoBudgetNotified: true,
			},
			patch: PatchCampaignRequest{
				GoalTotalDollars: floatPtr(6),
				NoBudgetNotified: boolPtr(true),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := tt.current
			applyPatchRequest(&current, tt.patch)
			if current.NoBudgetNotified != tt.want {
				t.Fatalf("NoBudgetNotified=%v want %v; campaign=%+v", current.NoBudgetNotified, tt.want, current)
			}
		})
	}
}
