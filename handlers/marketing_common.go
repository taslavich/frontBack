package handlers

import (
	"database/sql"
	"net/http"

	"github.com/lib/pq"
	"github.com/taslavich/frontBack/middleware"
	"github.com/taslavich/frontBack/models"
)

type MarketingHandler struct{ db *sql.DB }

func NewMarketingHandler(db *sql.DB) *MarketingHandler { return &MarketingHandler{db: db} }

func userIDFromCtx(r *http.Request) (string, bool) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	return uid, ok
}

func flattenIntervals(in [][2]string) []string {
	out := []string{}
	for _, v := range in {
		out = append(out, v[0], v[1])
	}
	return out
}

func parseIntervals(in []string) [][2]string {
	out := [][2]string{}
	for i := 0; i+1 < len(in); i += 2 {
		out = append(out, [2]string{in[i], in[i+1]})
	}
	return out
}

func scanCampaign(s interface {
	Scan(dest ...interface{}) error
}) (models.Campaign, error) {
	return scanCampaignRow(s)
}

func scanCampaignRow(s interface {
	Scan(dest ...interface{}) error
}) (models.Campaign, error) {
	var c models.Campaign
	var vertical []string
	var intervals []string
	err := s.Scan(&c.CampaignID, &c.UserID, &c.CampaignName, &c.FormatType, &c.BrandName, &c.H, &c.W, &c.Status, &c.TrafficType, pq.Array(&vertical), &c.PricingModel, &c.BasePriceCPM, &c.BasePriceCPC, &c.EvennessBySlotMode, &c.GoalTotalDollars, &c.CumDoneDollars, &c.StartTS, &c.EndTS, pq.Array(&intervals), &c.Country, &c.Language, &c.DeviceType, &c.OS, &c.Browser, &c.SiteID, &c.IP)
	c.Vertical = vertical
	c.ActiveIntervals = parseIntervals(intervals)
	return c, err
}

func scanCreative(s interface {
	Scan(dest ...interface{}) error
}) (models.Creative, error) {
	return scanCreativeRow(s)
}

func scanCreativeRow(s interface {
	Scan(dest ...interface{}) error
}) (models.Creative, error) {
	var c models.Creative
	err := s.Scan(&c.ID, &c.CampaignID, &c.CreativeName, &c.Link, &c.TrackersMacros, &c.W, &c.H, &c.S3FilePath, &c.FileFormat, &c.Title, &c.Description)
	return c, err
}

func scanTopup(s interface {
	Scan(dest ...interface{}) error
}) (models.UserTransaction, error) {
	return scanTopupRow(s)
}

func scanTopupRow(s interface {
	Scan(dest ...interface{}) error
}) (models.UserTransaction, error) {
	var t models.UserTransaction
	err := s.Scan(&t.ID, &t.UserID, &t.TransactionTime, &t.TransactionID, &t.PaymentMethod, &t.BonusAmount, &t.PromocodeID, &t.TransactionHash, &t.DepositAmount, &t.TotalBalanceIncrease, &t.Status, &t.Currency, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
