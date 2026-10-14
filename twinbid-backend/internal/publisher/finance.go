package publisher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

var trcWallet = regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)
var ercWallet = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func validWallet(method, wallet string) bool {
	if method == "usdt_trc20" {
		return trcWallet.MatchString(wallet) && validTronChecksum(wallet)
	}
	return method == "usdt_erc20" && ercWallet.MatchString(wallet)
}
func amount(v string) (decimal.Decimal, error) {
	a, e := decimal.NewFromString(v)
	if e != nil || a.Exponent() < -8 || len(v) > 32 {
		return decimal.Zero, httpx.BadRequest("amount must be a decimal string with <=8 decimal places")
	}
	return a, nil
}

type Payout struct {
	ID                string    `json:"id"`
	Amount            string    `json:"amount"`
	Method            string    `json:"method"`
	Wallet            string    `json:"wallet"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	TransferReference string    `json:"transfer_reference"`
}

const payoutCols = `id::text,amount::text,method,wallet,status,created_at,transfer_reference`

func scanPayout(row interface{ Scan(...any) error }) (Payout, error) {
	var p Payout
	e := row.Scan(&p.ID, &p.Amount, &p.Method, &p.Wallet, &p.Status, &p.CreatedAt, &p.TransferReference)
	return p, e
}
func (s *Service) Balance(ctx context.Context, user string) (map[string]string, error) {
	v := map[string]string{}
	var b, res string
	e := s.DB.QueryRowContext(ctx, `SELECT balance::text,reserved::text FROM publisher_accounts WHERE user_id=$1`, user).Scan(&b, &res)
	if errors.Is(e, sql.ErrNoRows) {
		b = "0"
		res = "0"
		e = nil
	}
	if e != nil {
		return nil, e
	}
	a, _ := decimal.NewFromString(b)
	q, _ := decimal.NewFromString(res)
	available := decimal.Max(decimal.Zero, a.Sub(q))
	var earned string
	if e := s.DB.QueryRowContext(ctx, `SELECT cumulative::text FROM publisher_revenue_checkpoints WHERE user_id=$1`, user).Scan(&earned); errors.Is(e, sql.ErrNoRows) {
		earned = "0"
	} else if e != nil {
		return nil, e
	}
	var paid string
	if e := s.DB.QueryRowContext(ctx, `SELECT COALESCE(sum(amount),0)::text FROM publisher_payouts WHERE user_id=$1 AND status='paid'`, user).Scan(&paid); e != nil {
		return nil, e
	}
	v["paid_out"] = paid
	v["cumulative_earned"] = earned
	v["revenue_status"] = s.RevenueStatus(ctx)
	v["balance"] = b
	v["reserved"] = res
	v["available_for_payout"] = available.String()
	return v, nil
}
func (s *Service) balance(w http.ResponseWriter, r *http.Request) {
	v, e := s.Balance(r.Context(), auth.UserID(r))
	reply(w, v, e)
}
func (s *Service) listPayouts(w http.ResponseWriter, r *http.Request) {
	limit, offset, e := pagination(r)
	if e != nil {
		reply(w, nil, e)
		return
	}
	rows, e := s.DB.QueryContext(r.Context(), "SELECT "+payoutCols+" FROM publisher_payouts WHERE user_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", auth.UserID(r), limit, offset)
	if e != nil {
		reply(w, nil, e)
		return
	}
	defer rows.Close()
	items := []Payout{}
	for rows.Next() {
		p, e := scanPayout(rows)
		if e != nil {
			reply(w, nil, e)
			return
		}
		items = append(items, p)
	}
	reply(w, map[string]any{"items": items, "limit": limit, "offset": offset}, rows.Err())
}
func (s *Service) createPayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount    string `json:"amount"`
		Method    string `json:"method"`
		Wallet    string `json:"wallet"`
		RequestID string `json:"request_id"`
	}
	if e := decode(w, r, &req); e != nil {
		reply(w, nil, e)
		return
	}
	a, e := amount(req.Amount)
	if e != nil {
		reply(w, nil, e)
		return
	}
	if a.LessThan(decimal.NewFromInt(50)) || !validWallet(req.Method, req.Wallet) {
		reply(w, nil, httpx.BadRequest("minimum payout $50; valid USDT wallet/network required"))
		return
	}
	if _, e = uuid.Parse(req.RequestID); e != nil {
		reply(w, nil, httpx.BadRequest("request_id UUID required"))
		return
	}
	user := auth.UserID(r)
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		reply(w, nil, e)
		return
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(r.Context(), `INSERT INTO publisher_accounts(user_id) VALUES($1) ON CONFLICT DO NOTHING`, user)
	if e != nil {
		reply(w, nil, e)
		return
	}
	var available string
	if e = tx.QueryRowContext(r.Context(), `SELECT (balance-reserved)::text FROM publisher_accounts WHERE user_id=$1 FOR UPDATE`, user).Scan(&available); e != nil {
		reply(w, nil, e)
		return
	}
	old, err := scanPayout(tx.QueryRowContext(r.Context(), "SELECT "+payoutCols+" FROM publisher_payouts WHERE user_id=$1 AND request_id=$2", user, req.RequestID))
	if err == nil {
		if old.Amount != a.StringFixed(8) || old.Method != req.Method || old.Wallet != req.Wallet {
			reply(w, nil, httpx.Conflict("request_id reused with different payout"))
			return
		}
		reply(w, old, nil)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		reply(w, nil, err)
		return
	}
	funds, _ := decimal.NewFromString(available)
	if funds.LessThan(a) {
		reply(w, nil, httpx.Conflict("insufficient available balance"))
		return
	}
	p, e := scanPayout(tx.QueryRowContext(r.Context(), `INSERT INTO publisher_payouts(id,user_id,request_id,amount,method,wallet) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+payoutCols, uuid.NewString(), user, req.RequestID, a.String(), req.Method, req.Wallet))
	if e == nil {
		_, e = tx.ExecContext(r.Context(), `UPDATE publisher_accounts SET reserved=reserved+$2 WHERE user_id=$1`, user, a.String())
	}
	if e == nil {
		e = tx.Commit()
	}
	reply(w, p, e)
}
func (s *Service) transition(ctx context.Context, id, user, status, reference string) (Payout, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Payout{}, e
	}
	defer tx.Rollback()
	var owner string
	if e = tx.QueryRowContext(ctx, `SELECT user_id::text FROM publisher_payouts WHERE id=$1`, id).Scan(&owner); e != nil {
		return Payout{}, e
	}
	if user != "" && owner != user {
		return Payout{}, sql.ErrNoRows
	}
	// Uniform account -> payout lock order avoids deadlocks with creation.
	var balance, reserved string
	if e = tx.QueryRowContext(ctx, `SELECT balance::text,reserved::text FROM publisher_accounts WHERE user_id=$1 FOR UPDATE`, owner).Scan(&balance, &reserved); e != nil {
		return Payout{}, e
	}
	p, e := scanPayout(tx.QueryRowContext(ctx, "SELECT "+payoutCols+" FROM publisher_payouts WHERE id=$1 FOR UPDATE", id))
	if e != nil {
		return p, e
	}
	if p.Status == status {
		if status == "paid" && p.TransferReference != reference {
			return p, httpx.Conflict("paid transfer reference differs")
		}
		return p, nil
	}
	if p.Status != "pending" {
		return p, httpx.Conflict("payout already finalized")
	}
	if status == "paid" && strings.TrimSpace(reference) == "" {
		return p, httpx.BadRequest("confirmed transfer_reference required")
	}
	if status == "paid" {
		available, _ := decimal.NewFromString(balance)
		held, _ := decimal.NewFromString(reserved)
		if available.LessThan(held) {
			return p, httpx.Conflict("payout blocked: revenue correction left reserved funds uncovered")
		}
	}
	q := `UPDATE publisher_accounts SET reserved=reserved-$2 WHERE user_id=$1`
	if status == "paid" {
		q = `UPDATE publisher_accounts SET reserved=reserved-$2,balance=balance-$2 WHERE user_id=$1`
	}
	if _, e = tx.ExecContext(ctx, q, owner, p.Amount); e != nil {
		return p, e
	}
	p, e = scanPayout(tx.QueryRowContext(ctx, `UPDATE publisher_payouts SET status=$2,transfer_reference=$3,updated_at=now(),notified_at=NULL WHERE id=$1 RETURNING `+payoutCols, id, status, reference))
	if e == nil {
		e = tx.Commit()
	}
	return p, e
}
func (s *Service) cancelPayout(w http.ResponseWriter, r *http.Request) {
	reply(w, nil, httpx.Forbidden("only operator may cancel a payout; contact support"))
}
func (s *Service) payoutDecision(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		reply(w, nil, httpx.Forbidden("admin required"))
		return
	}
	var req struct {
		Status            string `json:"status"`
		TransferReference string `json:"transfer_reference"`
	}
	if e := decode(w, r, &req); e != nil {
		reply(w, nil, e)
		return
	}
	if req.Status != "paid" && req.Status != "canceled" {
		reply(w, nil, httpx.BadRequest("paid or canceled required"))
		return
	}
	id := chi.URLParam(r, "id")
	if _, e := uuid.Parse(id); e != nil {
		reply(w, nil, httpx.NotFound("not found"))
		return
	}
	v, e := s.transition(r.Context(), id, "", req.Status, req.TransferReference)
	reply(w, v, e)
}

// The shared aggregate is the only authority for POP publisher earnings.
// Keep a clear refusal for former integrations; accepting signed parallel
// credits here would double-count the same traffic.
func (s *Service) Financial(w http.ResponseWriter, r *http.Request) {
	reply(w, nil, httpx.Conflict("publisher revenue is synchronized automatically from agg_stats; external credits are disabled"))
}

func validTronChecksum(address string) bool {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	n := new(big.Int)
	base := big.NewInt(58)
	for _, char := range address {
		digit := strings.IndexRune(alphabet, char)
		if digit < 0 {
			return false
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(digit)))
	}
	data := n.Bytes()
	if len(data) != 25 || data[0] != 0x41 {
		return false
	}
	first := sha256.Sum256(data[:21])
	second := sha256.Sum256(first[:])
	return bytes.Equal(data[21:], second[:4])
}
