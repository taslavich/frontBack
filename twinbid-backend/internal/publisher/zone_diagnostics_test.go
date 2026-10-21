package publisher

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"
	"time"
)

type zoneCodeRows struct{ done bool }

func (*zoneCodeRows) Columns() []string { return []string{"ad_code_id"} }
func (*zoneCodeRows) Close() error      { return nil }
func (r *zoneCodeRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	v[0] = "trusted"
	r.done = true
	return nil
}

type zoneReporterFixture struct {
	user  string
	codes []string
}

func (*zoneReporterFixture) PublisherQuery(context.Context, string, StatsRequest) (Report, error) {
	return Report{}, nil
}
func (p *zoneReporterFixture) PublisherZoneDiagnostics(_ context.Context, u string, c []string, l, o int, _ time.Time) (ZoneDiagnostics, error) {
	p.user = u
	p.codes = c
	return ZoneDiagnostics{Rows: []ZoneDiagnostic{}, Limit: l, Offset: o}, nil
}
func TestZoneOwnedActiveRegistry(t *testing.T) {
	db := sql.OpenDB(patchTrafficConnector{query: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		if !strings.Contains(q, "user_id=$1 AND is_active AND archived_at IS NULL") || a[0].Value != "owner" || a[1].Value != "trusted" {
			t.Fatal(q, a)
		}
		return &zoneCodeRows{}, nil
	}})
	defer db.Close()
	provider := &zoneReporterFixture{}
	s := Service{DB: db, Reports: provider}
	if _, e := s.zoneReport(context.Background(), "owner", "trusted", 50, 0); e != nil {
		t.Fatal(e)
	}
	if provider.user != "owner" || len(provider.codes) != 1 || provider.codes[0] != "trusted" {
		t.Fatal(provider)
	}
}
func TestZoneStatus(t *testing.T) {
	for _, tc := range []struct {
		a, b uint64
		want string
	}{{1, 0, "active"}, {0, 1, "active"}, {1, 10, "active"}, {0, 0, "problem"}} {
		if ZoneStatus(tc.a, tc.b) != tc.want {
			t.Fatal(tc)
		}
	}
}
