package publisher

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/config"
)

// Exercise the real handler and SQL arguments without requiring a running PG/Redis.
func TestPatchTrafficTypeHandler(t *testing.T) {
	for _, tc := range []struct {
		name, stored, field, want string
		status                    int
	}{
		{"omit_adult", "adult", "", "adult", 200},
		{"omit_mainstream", "mainstream", "", "mainstream", 200},
		{"set_mainstream", "adult", `,"traffic_type":"mainstream"`, "mainstream", 200},
		{"set_adult", "mainstream", `,"traffic_type":"adult"`, "adult", 200},
		{"null", "adult", `,"traffic_type":null`, "", 400},
		{"empty", "adult", `,"traffic_type":""`, "", 400},
		{"mixed", "adult", `,"traffic_type":"mixed"`, "", 400},
		{"unknown", "adult", `,"traffic_type":"other"`, "", 400},
		{"case", "adult", `,"traffic_type":"Adult"`, "", 400},
		{"whitespace", "adult", `,"traffic_type":"adult "`, "", 400},
		{"boolean", "adult", `,"traffic_type":true`, "", 400},
		{"number", "adult", `,"traffic_type":123`, "", 400},
		{"array", "adult", `,"traffic_type":[]`, "", 400},
		{"object", "adult", `,"traffic_type":{}`, "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const id = "c88c2e4a-22de-4fba-808f-63c38027eafd"
			const owner = "8958e009-80bb-4dc5-9a96-bfd717c31a80"
			values := []driver.Value{id, owner, "tb_test", "original", "popunder", tc.stored, "all", true, false, int64(1), float64(10), nil, int64(1), time.Unix(0, 0).UTC(), int64(0)}
			reads, writes := 0, 0
			connector := patchTrafficConnector{query: func(query string, args []driver.NamedValue) (driver.Rows, error) {
				switch {
				case strings.HasPrefix(query, "SELECT "):
					reads++
					if len(args) != 2 || args[0].Value != id || args[1].Value != owner {
						t.Fatalf("owner lookup arguments: %v", args)
					}
				case strings.HasPrefix(query, "UPDATE ad_codes "):
					writes++
					got := make([]any, len(args))
					for i, arg := range args {
						got[i] = arg.Value
					}
					want := []any{id, owner, "renamed", tc.want, "all", false, int64(1), float64(10), nil, int64(1)}
					if tc.status != 200 || !reflect.DeepEqual(got, want) {
						t.Fatalf("unexpected UPDATE arguments: got %v, want %v", got, want)
					}
					values = append([]driver.Value(nil), values...)
					values[3], values[5], values[7], values[12] = "renamed", tc.want, false, int64(2)
				default:
					t.Fatalf("unexpected query: %s", query)
				}
				return &patchTrafficRows{values: values}, nil
			}}
			pg := sql.OpenDB(connector)
			t.Cleanup(func() { pg.Close() })
			s := &Service{DB: pg}
			h := chi.NewRouter()
			h.Use(auth.Middleware(auth.NewService(nil, config.JWTConfig{Secret: "test"}, config.SMTPConfig{})))
			h.Patch("/api/publisher/ad-codes/{id}", s.patchCode)
			body := json.RawMessage(`{"version":1,"name":"renamed","is_active":false` + tc.field + `}`)
			w := req(h, owner, http.MethodPatch, "/api/publisher/ad-codes/"+id, body)
			if w.Code != tc.status || reads != 1 {
				t.Fatalf("status=%d reads=%d body=%s", w.Code, reads, w.Body.String())
			}
			if tc.status == 400 {
				if writes != 0 || !strings.Contains(w.Body.String(), "traffic_type") {
					t.Fatalf("invalid type wrote=%d body=%s", writes, w.Body.String())
				}
				return
			}
			var response struct {
				Data Code `json:"data"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
				t.Fatal(e)
			}
			if writes != 1 || response.Data.TrafficType != tc.want || response.Data.Name != "renamed" || response.Data.Active || response.Data.Version != 2 {
				t.Fatalf("writes=%d response=%+v", writes, response.Data)
			}
		})
	}
}

type patchTrafficConnector struct {
	query func(string, []driver.NamedValue) (driver.Rows, error)
}

func (c patchTrafficConnector) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (c patchTrafficConnector) Driver() driver.Driver                        { return c }
func (c patchTrafficConnector) Open(string) (driver.Conn, error)             { return c, nil }
func (c patchTrafficConnector) Close() error                                 { return nil }
func (c patchTrafficConnector) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (c patchTrafficConnector) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected Begin")
}
func (c patchTrafficConnector) QueryContext(_ context.Context, q string, a []driver.NamedValue) (driver.Rows, error) {
	return c.query(q, a)
}

type patchTrafficRows struct{ values []driver.Value }

func (*patchTrafficRows) Columns() []string { return strings.Split(codeColumns, ",") }
func (*patchTrafficRows) Close() error      { return nil }
func (r *patchTrafficRows) Next(dest []driver.Value) error {
	if r.values == nil {
		return io.EOF
	}
	copy(dest, r.values)
	r.values = nil
	return nil
}
