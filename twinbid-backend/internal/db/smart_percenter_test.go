package db

import (
	"strings"
	"testing"
)

func TestSmartPercenterSchemaIncludesPromoGenerationAndAtomicTriggerReplacement(t *testing.T) {
	joined := strings.Join(smartPercenterSchemaQueries(), "\n")
	for _, want := range []string{
		"users ADD COLUMN IF NOT EXISTS promo_generation",
		"users ADD COLUMN IF NOT EXISTS promo_spend_synced",
		"SET promo_spend_synced = cum_done_dollars",
		"ALTER COLUMN promo_spend_synced SET NOT NULL",
		"OLD.promo_spend_remaining <= 0",
		"NEW.promo_spend_remaining > 0",
		"NEW.promo_generation := OLD.promo_generation + 1",
		"CREATE OR REPLACE TRIGGER users_promo_revision_bump",
		"BEFORE UPDATE OF promo_spend_remaining, promo_revision, promo_generation ON users",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("smart percenter schema missing %q", want)
		}
	}
	if strings.Contains(joined, "adv_promo_spend_events") {
		t.Fatalf("retired realtime promo-spend ledger must not be created by schema bootstrap: %s", joined)
	}
	if strings.Contains(joined, "DROP TRIGGER") {
		t.Fatal("promo trigger migration must not create a DROP/CREATE gap")
	}
}
