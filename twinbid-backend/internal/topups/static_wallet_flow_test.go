package topups

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/tronscan"
)

type scriptedWalletVerifier struct {
	results      []tronscan.VerificationState
	lookupErr    error
	starts, ends []time.Time
}

func (v *scriptedWalletVerifier) Enabled() bool { return true }
func (v *scriptedWalletVerifier) VerifyUSDTTransfer(_ context.Context, hash string, micro *big.Int) (tronscan.Verification, error) {
	v.starts = append(v.starts, time.Now())
	if hash != "hash-1" || micro.Cmp(big.NewInt(100000000)) != 0 {
		return tronscan.Verification{}, fmt.Errorf("unexpected verification input")
	}
	index := len(v.starts) - 1
	if index >= len(v.results) {
		return tronscan.Verification{}, fmt.Errorf("extra verification attempt")
	}
	v.ends = append(v.ends, time.Now())
	return tronscan.Verification{State: v.results[index], AmountMicro: new(big.Int).Set(micro), Confirmations: 4, Raw: json.RawMessage(`{"hash":"hash-1"}`)}, v.lookupErr
}
func pendingPayment() models.UserTransaction {
	hash := "hash-1"
	return models.UserTransaction{ID: "topup-1", UserID: "user-1", TransactionID: "tx-1", PaymentChannel: PaymentChannelStaticWallet,
		PaymentMethod: "usdt_trc20", TransactionHash: &hash, DepositAmount: 100, TotalBalanceIncrease: 100, Currency: "USD", Status: models.TopupPending}
}
func approvedPayment() models.UserTransaction {
	item := pendingPayment()
	now := time.Now().UTC()
	status := "tronscan_verified"
	item.Status = models.TopupApproved
	item.CreditedAt = &now
	item.ProviderStatus = &status
	return item
}
func successfulCreditSteps() []paymentDBStep {
	return []paymentDBStep{
		{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(pendingPayment())),
		{kind: "exec", match: "pg_advisory_xact_lock", args: func(args []driver.NamedValue) error {
			if args[0].Value != "static-wallet:hash-1" {
				return fmt.Errorf("wrong hash lock: %v", args)
			}
			return nil
		}},
		paymentQuery("SELECT EXISTS", []driver.Value{false}),
		paymentQuery("SET status='approved'", topupRow(approvedPayment())),
		{kind: "query", match: "promo_spend_remaining = promo_spend_remaining + $3", rows: [][]driver.Value{paymentUserRow()}, args: func(args []driver.NamedValue) error {
			if args[1].Value != float64(100) || args[2].Value != float64(0) {
				return fmt.Errorf("wrong credit/promo amounts: %v", args)
			}
			return nil
		}},
		{kind: "exec", match: "antiperekrut_blocked = FALSE"}, {kind: "commit"},
		paymentQuery("FROM users WHERE id=$1", paymentUserRow()),
	}
}
func failedLookupSteps() []paymentDBStep {
	return []paymentDBStep{{kind: "exec", match: "provider_check_attempts=provider_check_attempts+1"}, paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(pendingPayment()))}
}
func submissionSteps(path string) []paymentDBStep {
	if path == "create" {
		return []paymentDBStep{{kind: "begin"}, paymentQuery("INSERT INTO user_transactions", topupRow(pendingPayment())), {kind: "commit"}}
	}
	item := pendingPayment()
	item.TransactionHash = nil
	return []paymentDBStep{{kind: "exec", match: "invoice_expires_at <= $1"}, paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(item)), paymentQuery("SET transaction_hash=$3", topupRow(pendingPayment()))}
}

type paymentNotifications struct {
	mu       sync.Mutex
	requests []bot.PaymentModerationRequest
}

func (n *paymentNotifications) snapshot() []bot.PaymentModerationRequest {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]bot.PaymentModerationRequest(nil), n.requests...)
}
func newWalletFlowService(t *testing.T, db *sql.DB, verifier *scriptedWalletVerifier, notificationStatus int) (*Service, *paymentNotifications) {
	t.Helper()
	messages := &paymentNotifications{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/payments/moderation" || r.Header.Get("X-Bot-Secret") != "test-secret" {
			t.Errorf("unexpected notification request: %s", r.URL.Path)
		}
		var req bot.PaymentModerationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		messages.mu.Lock()
		messages.requests = append(messages.requests, req)
		messages.mu.Unlock()
		w.WriteHeader(notificationStatus)
	}))
	t.Cleanup(server.Close)
	profileRepo := profile.NewRepository(db)
	svc := NewService(NewRepository(db), nil, nil, profileRepo, profile.NewService(profileRepo), config.BotConfig{BaseURL: server.URL, InternalSecret: "test-secret"})
	svc.ConfigureStaticWalletAutoApproval(verifier, 15*time.Second)
	return svc, messages
}
func submitPayment(ctx context.Context, svc *Service, path string) (models.UserTransaction, error) {
	hash := "hash-1"
	if path == "create" {
		return svc.Create(ctx, "user-1", CreateTopupRequest{PaymentChannel: PaymentChannelStaticWallet, PaymentMethod: "usdt_trc20", DepositAmount: 100, TransactionHash: &hash})
	}
	return svc.Patch(ctx, "user-1", "topup-1", PatchTopupRequest{TransactionHashSet: true, TransactionHash: &hash})
}
func assertAttemptTiming(t *testing.T, verifier *scriptedWalletVerifier, start time.Time, attempts int) {
	t.Helper()
	if len(verifier.starts) != attempts {
		t.Fatalf("verification calls=%d want=%d", len(verifier.starts), attempts)
	}
	if verifier.starts[0].Sub(start) >= time.Second {
		t.Fatal("unexpected delay before first attempt")
	}
	for i := 1; i < attempts; i++ {
		if gap := verifier.starts[i].Sub(verifier.ends[i-1]); gap < time.Second {
			t.Fatalf("retry gap=%v, want >=1s after result", gap)
		}
	}
}
func TestImmediateStaticWalletSubmissionApprovesOnAttemptsOneTwoThree(t *testing.T) {
	for _, path := range []string{"create", "patch"} {
		for attempts := 1; attempts <= 3; attempts++ {
			t.Run(fmt.Sprintf("%s/attempt%d", path, attempts), func(t *testing.T) {
				steps := submissionSteps(path)
				verifier := &scriptedWalletVerifier{}
				for i := 1; i < attempts; i++ {
					steps = append(steps, failedLookupSteps()...)
					verifier.results = append(verifier.results, tronscan.VerificationPending)
				}
				steps = append(steps, successfulCreditSteps()...)
				verifier.results = append(verifier.results, tronscan.VerificationVerified)
				svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
				start := time.Now()
				result, err := submitPayment(context.Background(), svc, path)
				if err != nil {
					t.Fatal(err)
				}
				if result.Status != models.TopupApproved || result.CreditedAt == nil {
					t.Fatalf("not credited: %+v", result)
				}
				assertAttemptTiming(t, verifier, start, attempts)
				messages := notifications.snapshot()
				if len(messages) != 1 || !messages[0].AutoApproved || messages[0].TotalBalanceIncrease != 100 {
					t.Fatalf("unexpected notifications: %+v", messages)
				}
			})
		}
	}
}
func TestImmediateStaticWalletAllFailuresSendOneManualCard(t *testing.T) {
	for _, path := range []string{"create", "patch"} {
		for _, failure := range []string{"pending", "invalid", "api error"} {
			t.Run(path+"/"+failure, func(t *testing.T) {
				steps := submissionSteps(path)
				verifier := &scriptedWalletVerifier{}
				state := tronscan.VerificationPending
				if failure == "invalid" {
					state = tronscan.VerificationInvalid
				}
				if failure == "api error" {
					verifier.lookupErr = errors.New("temporary lookup failure")
				}
				for i := 0; i < 3; i++ {
					steps = append(steps, failedLookupSteps()...)
					verifier.results = append(verifier.results, state)
				}
				steps = append(steps, paymentQuery("FROM users WHERE id=$1", paymentUserRow()))
				svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
				start := time.Now()
				result, err := submitPayment(context.Background(), svc, path)
				if err != nil {
					t.Fatal(err)
				}
				if result.Status != models.TopupPending || result.CreditedAt != nil {
					t.Fatalf("unexpected credit: %+v", result)
				}
				assertAttemptTiming(t, verifier, start, 3)
				messages := notifications.snapshot()
				if len(messages) != 1 || messages[0].AutoApproved {
					t.Fatalf("unexpected notifications: %+v", messages)
				}
			})
		}
	}
}
func TestAutomaticCreditNotificationFailureDoesNotUndoCredit(t *testing.T) {
	steps := append(submissionSteps("create"), successfulCreditSteps()...)
	verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationVerified}}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusInternalServerError)
	result, err := submitPayment(context.Background(), svc, "create")
	if err != nil || result.Status != models.TopupApproved || result.CreditedAt == nil {
		t.Fatalf("financial commit lost: result=%+v err=%v", result, err)
	}
	if messages := notifications.snapshot(); len(messages) != 1 || !messages[0].AutoApproved {
		t.Fatalf("manual fallback after credit: %+v", messages)
	}
}
func TestImmediateCreditRaceDoesNotEmitFalseAutoApproval(t *testing.T) {
	for _, state := range []tronscan.VerificationState{tronscan.VerificationVerified, tronscan.VerificationPending} {
		t.Run(string(state), func(t *testing.T) {
			steps := submissionSteps("create")
			if state == tronscan.VerificationVerified {
				steps = append(steps, paymentDBStep{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(approvedPayment())), paymentDBStep{kind: "rollback"})
			} else {
				steps = append(steps, paymentDBStep{kind: "exec", match: "provider_check_attempts=provider_check_attempts+1"}, paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(approvedPayment())))
			}
			verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{state}}
			svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
			result, err := submitPayment(context.Background(), svc, "create")
			if err != nil || result.Status != models.TopupApproved {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(notifications.snapshot()) != 0 || len(verifier.starts) != 1 {
				t.Fatal("race caused extra attempt or false notification")
			}
		})
	}
}
func TestBackgroundCreditSendsAutoApprovedNotification(t *testing.T) {
	steps := []paymentDBStep{paymentQuery("LIMIT $1", topupRow(pendingPayment()))}
	steps = append(steps, successfulCreditSteps()...)
	verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationVerified}}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	result, err := svc.ReconcilePendingStaticWallets(context.Background(), 50, 0, time.Second)
	if err != nil || result.Checked != 1 || result.Approved != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if messages := notifications.snapshot(); len(messages) != 1 || !messages[0].AutoApproved {
		t.Fatalf("notifications=%+v", messages)
	}
}
func TestManualApproveDistinguishesAlreadyApprovedFromNotReady(t *testing.T) {
	for _, state := range []string{"approved", "credited", "not ready"} {
		t.Run(state, func(t *testing.T) {
			item := pendingPayment()
			want := "topup is already approved"
			switch state {
			case "approved":
				item.Status = models.TopupApproved
			case "credited":
				now := time.Now()
				item.CreditedAt = &now
			default:
				item.TransactionHash = nil
				want = "topup is not ready for approval"
			}
			db := newPaymentDB(t, []paymentDBStep{{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(item)), {kind: "rollback"}})
			svc := NewService(NewRepository(db), nil, nil, nil, nil, config.BotConfig{})
			_, err := svc.Approve(context.Background(), item.UserID, item.ID)
			var he httpx.HTTPError
			if !errors.As(err, &he) || he.Status != http.StatusConflict || he.Message != want {
				t.Fatalf("error=%v want 409 %q", err, want)
			}
		})
	}
}
func TestDuplicateHashNeverCreditsAfterThreeAttempts(t *testing.T) {
	steps := submissionSteps("create")
	verifier := &scriptedWalletVerifier{}
	for i := 0; i < 3; i++ {
		verifier.results = append(verifier.results, tronscan.VerificationVerified)
		steps = append(steps, paymentDBStep{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(pendingPayment())),
			paymentDBStep{kind: "exec", match: "pg_advisory_xact_lock"}, paymentQuery("SELECT EXISTS", []driver.Value{true}),
			paymentDBStep{kind: "rollback"}, paymentDBStep{kind: "exec", match: "provider_check_attempts=provider_check_attempts+1"},
			paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(pendingPayment())))
	}
	steps = append(steps, paymentQuery("FROM users WHERE id=$1", paymentUserRow()))
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	result, err := submitPayment(context.Background(), svc, "create")
	if err != nil || result.Status != models.TopupPending || result.CreditedAt != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if messages := notifications.snapshot(); len(messages) != 1 || messages[0].AutoApproved {
		t.Fatalf("notifications=%+v", messages)
	}
}
func TestImmediateRetryCanRecoverFromCreditFailure(t *testing.T) {
	steps := submissionSteps("create")
	steps = append(steps, paymentDBStep{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(pendingPayment())), paymentDBStep{kind: "exec", match: "pg_advisory_xact_lock"},
		paymentQuery("SELECT EXISTS", []driver.Value{false}), paymentDBStep{kind: "query", match: "SET status='approved'", err: errors.New("temporary write failure")},
		paymentDBStep{kind: "rollback"}, paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(pendingPayment())))
	steps = append(steps, successfulCreditSteps()...)
	verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationVerified, tronscan.VerificationVerified}}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	start := time.Now()
	result, err := submitPayment(context.Background(), svc, "create")
	if err != nil || result.Status != models.TopupApproved {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertAttemptTiming(t, verifier, start, 2)
	if messages := notifications.snapshot(); len(messages) != 1 || !messages[0].AutoApproved {
		t.Fatalf("notifications=%+v", messages)
	}
}
func TestImmediateRetryCancellationStopsWait(t *testing.T) {
	steps := failedLookupSteps()
	verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationPending}}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := svc.attemptImmediateStaticWalletAutoApproval(ctx, pendingPayment())
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) >= time.Second || len(verifier.starts) != 1 || len(notifications.snapshot()) != 0 {
		t.Fatalf("cancellation failed: err=%v attempts=%d", err, len(verifier.starts))
	}
}

func TestBackgroundCreditRaceDoesNotNotifyOrCountAnotherFlowsCredit(t *testing.T) {
	steps := []paymentDBStep{paymentQuery("LIMIT $1", topupRow(pendingPayment())), {kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(approvedPayment())), {kind: "rollback"}}
	verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationVerified}}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	result, err := svc.ReconcilePendingStaticWallets(context.Background(), 50, 0, time.Second)
	if err != nil || result.Checked != 1 || result.Approved != 0 || len(notifications.snapshot()) != 0 {
		t.Fatalf("result=%+v err=%v notifications=%+v", result, err, notifications.snapshot())
	}
}

func TestRepeatedHashSubmissionDoesNotDuplicateManualCard(t *testing.T) {
	steps := []paymentDBStep{{kind: "exec", match: "invoice_expires_at <= $1"}, paymentQuery("WHERE id=$2 AND user_id=$1", topupRow(pendingPayment())), paymentQuery("SET transaction_hash=$3", topupRow(pendingPayment()))}
	verifier := &scriptedWalletVerifier{}
	for i := 0; i < 3; i++ {
		steps = append(steps, failedLookupSteps()...)
		verifier.results = append(verifier.results, tronscan.VerificationPending)
	}
	svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
	result, err := submitPayment(context.Background(), svc, "patch")
	if err != nil || result.Status != models.TopupPending || len(verifier.starts) != 3 || len(notifications.snapshot()) != 0 {
		t.Fatalf("result=%+v err=%v attempts=%d", result, err, len(verifier.starts))
	}
}

func TestCreditHashOrAmountChangeNeverAutoCredits(t *testing.T) {
	for _, change := range []string{"hash", "amount"} {
		t.Run(change, func(t *testing.T) {
			item := pendingPayment()
			if change == "hash" {
				hash := "different-hash"
				item.TransactionHash = &hash
			} else {
				item.DepositAmount = 200
			}
			steps := []paymentDBStep{{kind: "begin"}, paymentQuery("FOR UPDATE", topupRow(item))}
			if change == "amount" {
				steps = append(steps, paymentDBStep{kind: "exec", match: "pg_advisory_xact_lock"}, paymentQuery("SELECT EXISTS", []driver.Value{false}))
			}
			steps = append(steps, paymentDBStep{kind: "rollback"})
			verifier := &scriptedWalletVerifier{results: []tronscan.VerificationState{tronscan.VerificationVerified}}
			svc, notifications := newWalletFlowService(t, newPaymentDB(t, steps), verifier, http.StatusOK)
			_, _, newlyCredited, err := svc.verifyAndMaybeCreditStaticWallet(context.Background(), pendingPayment(), time.Second)
			if err == nil || newlyCredited || len(notifications.snapshot()) != 0 {
				t.Fatalf("changed %s credited: newly=%v err=%v", change, newlyCredited, err)
			}
		})
	}
}
