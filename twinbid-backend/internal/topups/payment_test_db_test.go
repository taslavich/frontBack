package topups

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"twinbid-backend/internal/models"
)

// A strict, ordered database/sql driver exercises the actual repository and
// creditLockedTopup without adding a test dependency or requiring production DBs.
// Unexpected writes fail rather than being silently accepted.
type paymentDBStep struct {
	kind  string
	match string
	rows  [][]driver.Value
	err   error
	args  func([]driver.NamedValue) error
}

type paymentTestDB struct {
	mu    sync.Mutex
	steps []paymentDBStep
}

type paymentConnector struct{ db *paymentTestDB }
type paymentConn struct{ db *paymentTestDB }
type paymentDriver struct{}
type paymentTx struct{ db *paymentTestDB }
type paymentRows struct {
	rows    [][]driver.Value
	columns []string
}

func (paymentDriver) Open(string) (driver.Conn, error) { return nil, fmt.Errorf("use connector") }
func (c paymentConnector) Driver() driver.Driver       { return paymentDriver{} }
func (c paymentConnector) Connect(context.Context) (driver.Conn, error) {
	return &paymentConn{c.db}, nil
}
func (c *paymentConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected Prepare")
}
func (c *paymentConn) Close() error { return nil }
func (c *paymentConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *paymentConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	_, err := c.db.take("begin", "", nil)
	return paymentTx{c.db}, err
}
func (tx paymentTx) Commit() error   { _, err := tx.db.take("commit", "", nil); return err }
func (tx paymentTx) Rollback() error { _, err := tx.db.take("rollback", "", nil); return err }
func (c *paymentConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	_, err := c.db.take("exec", query, args)
	return driver.RowsAffected(1), err
}
func (c *paymentConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	step, err := c.db.take("query", query, args)
	if err != nil {
		return nil, err
	}
	count := len(strings.Split(returningTx, ","))
	if len(step.rows) > 0 {
		count = len(step.rows[0])
	}
	columns := make([]string, count)
	for i := range columns {
		columns[i] = fmt.Sprintf("column%d", i)
	}
	return &paymentRows{rows: step.rows, columns: columns}, nil
}
func (r *paymentRows) Columns() []string { return r.columns }
func (r *paymentRows) Close() error      { return nil }
func (r *paymentRows) Next(dst []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dst, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}
func (db *paymentTestDB) take(kind, query string, args []driver.NamedValue) (paymentDBStep, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(db.steps) == 0 {
		return paymentDBStep{}, fmt.Errorf("unexpected %s: %s", kind, query)
	}
	step := db.steps[0]
	if step.kind != kind || (step.match != "" && !strings.Contains(query, step.match)) {
		return paymentDBStep{}, fmt.Errorf("expected %s containing %q; got %s: %s", step.kind, step.match, kind, query)
	}
	db.steps = db.steps[1:]
	if step.args != nil {
		if err := step.args(args); err != nil {
			return step, err
		}
	}
	return step, step.err
}
func newPaymentDB(t *testing.T, steps []paymentDBStep) *sql.DB {
	t.Helper()
	script := &paymentTestDB{steps: steps}
	db := sql.OpenDB(paymentConnector{script})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		script.mu.Lock()
		defer script.mu.Unlock()
		if len(script.steps) > 0 {
			t.Errorf("%d unconsumed SQL operations; next: %+v", len(script.steps), script.steps[0])
		}
	})
	return db
}
func topupRow(item models.UserTransaction) []driver.Value {
	var hash, credited, providerStatus driver.Value
	if item.TransactionHash != nil {
		hash = *item.TransactionHash
	}
	if item.CreditedAt != nil {
		credited = *item.CreditedAt
	}
	if item.ProviderStatus != nil {
		providerStatus = *item.ProviderStatus
	}
	return []driver.Value{
		item.ID, item.UserID, time.Unix(0, 0), item.TransactionID, item.PaymentChannel, item.PaymentMethod,
		item.BonusAmount, nil, false, hash, item.DepositAmount, item.TotalBalanceIncrease, string(item.Status), item.Currency,
		nil, providerStatus, nil, nil, nil, nil, nil, nil, credited, nil, int64(0), nil, nil, nil, time.Unix(0, 0), time.Unix(0, 0),
	}
}
func paymentUserRow() []driver.Value {
	return []driver.Value{"user-1", "test", "test@example.com", "Test", nil, "", float64(100), float64(0), float64(100), "UTC", false, false, false, false, float64(0), false, "", ""}
}
func paymentQuery(match string, row []driver.Value) paymentDBStep {
	return paymentDBStep{kind: "query", match: match, rows: [][]driver.Value{row}}
}
