package config

import (
	"os"
	"testing"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

func TestTronScanDefaultReconcileIntervalIsThirtyMinutes(t *testing.T) {
	t.Setenv("TRONSCAN_RECONCILE_INTERVAL", "")
	if err := os.Unsetenv("TRONSCAN_RECONCILE_INTERVAL"); err != nil {
		t.Fatal(err)
	}
	var cfg TronScanConfig
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ReconcileInterval != 30*time.Minute {
		t.Fatalf("default interval=%v", cfg.ReconcileInterval)
	}
}
