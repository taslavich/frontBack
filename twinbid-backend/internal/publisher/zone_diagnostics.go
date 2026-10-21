package publisher

import (
	"context"
	"net/http"
	"time"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type ZoneDiagnostic struct {
	CodeID        string     `json:"ad_code_id"`
	ZoneID        string     `json:"zone_id"`
	SiteID        string     `json:"site_id"`
	Hostname      string     `json:"hostname"`
	Status        string     `json:"status"`
	Requests200   uint64     `json:"requests_200"`
	Requests204   uint64     `json:"requests_204"`
	Requests4xx   uint64     `json:"requests_4xx"`
	Requests5xx   uint64     `json:"requests_5xx"`
	LastRequestAt *time.Time `json:"last_request_at"`
}

// UndiscoveredCode is a code-level observation, never a fabricated zone or HTTP error.
type UndiscoveredCode struct {
	CodeID string `json:"ad_code_id"`
	Status string `json:"status"`
}
type ZoneDiagnostics struct {
	CodesWithoutZones uint64             `json:"codes_without_zones"`
	UndiscoveredCodes []UndiscoveredCode `json:"undiscovered_codes"`
	Rows              []ZoneDiagnostic   `json:"rows"`
	ActiveZones       uint64             `json:"active_zones"`
	ProblemZones      uint64             `json:"problem_zones"`
	Limit             int                `json:"limit"`
	Offset            int                `json:"offset"`
	WindowFrom        time.Time          `json:"window_from"`
	WindowTo          time.Time          `json:"window_to"`
	RefreshedAt       string             `json:"refreshed_at"`
}
type ZoneReporter interface {
	PublisherZoneDiagnostics(context.Context, string, []string, int, int, time.Time) (ZoneDiagnostics, error)
}

func ZoneStatus(n200, n204 uint64) string {
	if n200 > 0 || n204 > 0 {
		return "active"
	}
	return "problem"
}
func (s *Service) zoneReport(ctx context.Context, user, code string, limit, offset int) (ZoneDiagnostics, error) {
	// Active code ownership comes from the authoritative PG registry, never from
	// client-supplied IDs or from traffic alone. Archived/disabled codes are excluded.
	rows, e := s.DB.QueryContext(ctx, `SELECT ad_code_id::text FROM ad_codes WHERE user_id=$1 AND is_active AND archived_at IS NULL AND ($2='' OR ad_code_id::text=$2) ORDER BY ad_code_id`, user, code)
	if e != nil {
		return ZoneDiagnostics{}, e
	}
	codes := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return ZoneDiagnostics{}, e
		}
		codes = append(codes, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return ZoneDiagnostics{}, e
	}
	provider, ok := s.Reports.(ZoneReporter)
	if !ok {
		return ZoneDiagnostics{}, httpx.ServiceUnavailable("zone diagnostics unavailable")
	}
	return provider.PublisherZoneDiagnostics(ctx, user, codes, limit, offset, time.Now().UTC())
}
func (s *Service) zoneDiagnostics(w http.ResponseWriter, r *http.Request) {
	limit, offset, e := pagination(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	report, e := s.zoneReport(r.Context(), auth.UserID(r), "", limit, offset)
	reply(w, report, e)
}
