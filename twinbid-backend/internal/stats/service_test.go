package stats

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const (
	testUserID     = "11111111-1111-1111-1111-111111111111"
	testCampaignID = "22222222-2222-2222-2222-222222222222"
	testCreativeID = "33333333-3333-3333-3333-333333333333"
)

func TestQueryBuildsClickHouseSQLAndMapsResponse(t *testing.T) {
	db, cleanup := openStubDB(t, []stubExpectedQuery{
		{
			queryContains: []string{
				"geo AS bucket",
				"toUInt64(sum(impressions)) AS impressions",
				"round(sum(spend_views_table) + sum(spend_clicks_table), 2) AS spent",
				"FROM ads.agg_stats",
				"user_id = toUUID(?)",
				"campaign_id IN (toUUID(?))",
				"creative_id IN (toUUID(?))",
				"browser IN (?)",
				"device_type IN (?)",
				"event_date BETWEEN toDate(?) AND toDate(?)",
				"GROUP BY geo",
			},
			args:    []driver.Value{testUserID, "2026-05-01", "2026-05-06", testCampaignID, testCreativeID, "Chrome", "mobile"},
			columns: []string{"bucket", "impressions", "clicks", "spent", "ctr"},
			rows: [][]driver.Value{
				{"DE", int64(1000), int64(25), float64(12.34), float64(2.5)},
				{"US", int64(500), int64(10), float64(5.5), float64(2.0)},
			},
		},
		{
			queryContains: []string{
				"toUInt64(sum(impressions)) AS impressions",
				"round(sum(spend_views_table) + sum(spend_clicks_table), 2) AS spent",
				"FROM ads.agg_stats",
				"WHERE user_id = toUUID(?)",
			},
			args:    []driver.Value{testUserID, "2026-05-01", "2026-05-06", testCampaignID, testCreativeID, "Chrome", "mobile"},
			columns: []string{"impressions", "clicks", "spent", "ctr"},
			rows: [][]driver.Value{
				{int64(1500), int64(35), float64(17.84), float64(2.33)},
			},
		},
	})
	defer cleanup()

	svc := &Service{db: db, table: "ads.agg_stats"}
	got, err := svc.Query(context.Background(), testUserID, QueryRequest{
		From:        "2026-05-01",
		To:          "2026-05-06",
		CampaignIDs: []string{testCampaignID},
		CreativeIDs: []string{testCreativeID},
		GroupBy:     GroupByCountry,
		Filters: map[string][]string{
			string(FilterByDeviceType): {"mobile"},
			string(FilterByBrowser):    {"Chrome"},
		},
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	wantRows := map[string]Summary{
		"DE": {Impressions: 1000, Clicks: 25, Spent: 12.34, CTR: 2.5},
		"US": {Impressions: 500, Clicks: 10, Spent: 5.5, CTR: 2.0},
	}
	if !reflect.DeepEqual(got.Rows, wantRows) {
		t.Fatalf("Rows mismatch\nwant: %#v\n got: %#v", wantRows, got.Rows)
	}
	wantTotals := Summary{Impressions: 1500, Clicks: 35, Spent: 17.84, CTR: 2.33}
	if got.Totals != wantTotals {
		t.Fatalf("Totals mismatch: want %#v got %#v", wantTotals, got.Totals)
	}
}

func TestWhereValidatesAndBuildsDeterministicFilters(t *testing.T) {
	svc := &Service{}
	where, args, err := svc.where(QueryRequest{
		CampaignIDs: []string{testCampaignID},
		Filters: map[string][]string{
			string(FilterByOS):      {"Android"},
			string(FilterByCountry): {"DE", "US"},
		},
	}, testUserID, "2026-05-01", "2026-05-06")
	if err != nil {
		t.Fatalf("where returned error: %v", err)
	}

	wantWhere := "user_id = toUUID(?) AND event_date BETWEEN toDate(?) AND toDate(?) AND campaign_id IN (toUUID(?)) AND geo IN (?,?) AND os IN (?)"
	if where != wantWhere {
		t.Fatalf("where mismatch\nwant: %s\n got: %s", wantWhere, where)
	}
	wantArgs := []any{testUserID, "2026-05-01", "2026-05-06", testCampaignID, "DE", "US", "Android"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch\nwant: %#v\n got: %#v", wantArgs, args)
	}
}

func TestQueryValidationErrors(t *testing.T) {
	svc := &Service{}
	tests := []struct {
		name string
		req  QueryRequest
		user string
	}{
		{name: "invalid group", req: QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: "format"}, user: testUserID},
		{name: "invalid date", req: QueryRequest{From: "2026/05/01", To: "2026-05-06", GroupBy: GroupByDate}, user: testUserID},
		{name: "date range", req: QueryRequest{From: "2026-05-06", To: "2026-05-01", GroupBy: GroupByDate}, user: testUserID},
		{name: "invalid user", req: QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByDate}, user: "bad-user"},
		{name: "invalid campaign", req: QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByDate, CampaignIDs: []string{"bad-campaign"}}, user: testUserID},
		{name: "invalid filter", req: QueryRequest{From: "2026-05-01", To: "2026-05-06", GroupBy: GroupByDate, Filters: map[string][]string{"site_id": {"site"}}}, user: testUserID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Query(context.Background(), tt.user, tt.req)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeDates(t *testing.T) {
	from, to, err := normalizeDates("2026-05-01", "2026-05-06")
	if err != nil {
		t.Fatalf("normalizeDates returned error: %v", err)
	}
	if from != "2026-05-01" || to != "2026-05-06" {
		t.Fatalf("unexpected dates: %s %s", from, to)
	}
}

type stubExpectedQuery struct {
	queryContains []string
	args          []driver.Value
	columns       []string
	rows          [][]driver.Value
}

type stubState struct {
	t       *testing.T
	mu      sync.Mutex
	expects []stubExpectedQuery
	seen    int
}

var (
	stubStatesMu sync.Mutex
	stubStates   = map[string]*stubState{}
)

func init() {
	sql.Register("stats_stub", stubDriver{})
}

func openStubDB(t *testing.T, expects []stubExpectedQuery) (*sql.DB, func()) {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	state := &stubState{t: t, expects: expects}

	stubStatesMu.Lock()
	stubStates[name] = state
	stubStatesMu.Unlock()

	db, err := sql.Open("stats_stub", name)
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		state.mu.Lock()
		seen := state.seen
		state.mu.Unlock()
		if seen != len(expects) {
			t.Fatalf("expected %d queries, saw %d", len(expects), seen)
		}
		stubStatesMu.Lock()
		delete(stubStates, name)
		stubStatesMu.Unlock()
	}
	return db, cleanup
}

type stubDriver struct{}

func (stubDriver) Open(name string) (driver.Conn, error) {
	stubStatesMu.Lock()
	state := stubStates[name]
	stubStatesMu.Unlock()
	return &stubConn{state: state}, nil
}

type stubConn struct{ state *stubState }

func (c *stubConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *stubConn) Close() error                        { return nil }
func (c *stubConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *stubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.state.seen >= len(c.state.expects) {
		c.state.t.Fatalf("unexpected query: %s", query)
	}
	expect := c.state.expects[c.state.seen]
	c.state.seen++

	compactQuery := strings.Join(strings.Fields(query), " ")
	for _, needle := range expect.queryContains {
		if !strings.Contains(compactQuery, needle) {
			c.state.t.Fatalf("query missing %q\nquery: %s", needle, compactQuery)
		}
	}

	gotArgs := make([]driver.Value, len(args))
	for i, arg := range args {
		gotArgs[i] = arg.Value
	}
	if !reflect.DeepEqual(gotArgs, expect.args) {
		c.state.t.Fatalf("query args mismatch\nwant: %#v\n got: %#v", expect.args, gotArgs)
	}

	return &stubRows{columns: expect.columns, rows: expect.rows}, nil
}

func (c *stubConn) CheckNamedValue(*driver.NamedValue) error { return nil }

type stubRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *stubRows) Columns() []string { return r.columns }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}
