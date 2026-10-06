package app

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"twinbid-backend/internal/config"
	"twinbid-backend/internal/topups"
)

type tickerWalletReconciler struct {
	enabled bool
	calls   chan time.Time
}

func (s *tickerWalletReconciler) StaticWalletAutoApprovalEnabled() bool { return s.enabled }
func (s *tickerWalletReconciler) ReconcilePendingStaticWallets(context.Context, int, time.Duration, time.Duration) (topups.StaticWalletReconcileResult, error) {
	s.calls <- time.Now()
	return topups.StaticWalletReconcileResult{}, nil
}
func TestTronScanTickerRunsAtStartupThenEveryThirtyMinutes(t *testing.T) {
	for _, interval := range []time.Duration{0, 30 * time.Minute} {
		t.Run(interval.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer func() { cancel(); synctest.Wait() }()
				svc := &tickerWalletReconciler{enabled: true, calls: make(chan time.Time, 10)}
				start := time.Now()
				go runTronScanReconcileTicker(ctx, config.TronScanConfig{ReconcileInterval: interval}, svc)
				synctest.Wait()
				if len(svc.calls) != 1 {
					t.Fatalf("startup calls=%d", len(svc.calls))
				}
				if got := <-svc.calls; !got.Equal(start) {
					t.Fatalf("startup delayed by %v", got.Sub(start))
				}
				time.Sleep(30*time.Minute - time.Nanosecond)
				synctest.Wait()
				if len(svc.calls) != 0 {
					t.Fatal("reconcile ran before 30 minutes")
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				if len(svc.calls) != 1 {
					t.Fatalf("ticker calls=%d", len(svc.calls))
				}
				if got := <-svc.calls; got.Sub(start) != 30*time.Minute {
					t.Fatalf("ticker interval=%v", got.Sub(start))
				}
				time.Sleep(30 * time.Minute)
				synctest.Wait()
				if len(svc.calls) != 1 {
					t.Fatalf("second ticker calls=%d", len(svc.calls))
				}
				cancel()
				synctest.Wait()
			})
		})
	}
}
func TestTronScanTickerDisabledMakesNoRequests(t *testing.T) {
	svc := &tickerWalletReconciler{calls: make(chan time.Time, 1)}
	runTronScanReconcileTicker(context.Background(), config.TronScanConfig{}, svc)
	if len(svc.calls) != 0 {
		t.Fatal("disabled verifier made a request")
	}
}
