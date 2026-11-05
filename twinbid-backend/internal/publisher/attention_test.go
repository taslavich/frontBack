package publisher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type inventoryPages struct {
	count, failOffset int
	seen              []int
}

func (p *inventoryPages) PublisherZoneDiagnostics(ctx context.Context, user string, codes []string, limit, offset int, asof time.Time) (ZoneDiagnostics, error) {
	if user != "owner" || limit != 500 {
		panic("invalid owner/page")
	}
	p.seen = append(p.seen, offset)
	if offset == p.failOffset {
		return ZoneDiagnostics{}, errors.New("CH outage")
	}
	out := []ZoneDiagnostic{}
	old := time.Now().Add(-2 * time.Hour)
	for i := offset; i < p.count && i < offset+limit; i++ {
		out = append(out, ZoneDiagnostic{CodeID: "code", Hostname: fmt.Sprint(i), LastRequestAt: &old, Status: "problem"})
	}
	to := asof.Truncate(time.Second)
	return ZoneDiagnostics{Rows: out, WindowFrom: to.Add(-time.Hour), WindowTo: to}, nil
}
func TestAttentionPages(t *testing.T) {
	for _, n := range []int{0, 499, 500, 501, 1501} {
		p := &inventoryPages{count: n, failOffset: -1}
		inventory, e := attentionInventory(context.Background(), p, "owner", []string{"code"})
		if e != nil || len(inventory.problemsByCode["code"]) != n {
			t.Fatal(n, len(inventory.problemsByCode["code"]), e)
		}
		if inventory.windowTo.Sub(inventory.windowFrom) != time.Hour {
			t.Fatal("lost diagnostic period")
		}
		if n > 0 && inventory.problemsByCode["code"][n-1].Hostname != fmt.Sprint(n-1) {
			t.Fatal("lost site details")
		}
		want := n/500 + 1
		if len(p.seen) != want {
			t.Fatal(n, p.seen)
		}
	}
	p := &inventoryPages{count: 1000, failOffset: 500}
	if _, e := attentionInventory(context.Background(), p, "owner", []string{"code"}); e == nil {
		t.Fatal("partial outage accepted")
	}
}

func TestPublisherTelegramNoticeIncludesOperatorDetails(t *testing.T) {
	to := time.Date(2026, 11, 5, 9, 10, 21, 0, time.UTC)
	inventory := publisherAttentionInventory{windowFrom: to.Add(-time.Hour), windowTo: to}
	last := to.Add(-10 * time.Minute)
	zones := make([]ZoneDiagnostic, 9)
	for index := range zones {
		zones[index] = ZoneDiagnostic{Hostname: fmt.Sprintf("site-%d.example", index)}
	}
	zones[0].Requests4xx, zones[0].Requests5xx, zones[0].LastRequestAt = 3, 2, &last
	message := inventory.telegramAttentionText("owner-id", "code-id", zones)
	period := "Checked period (UTC): 2026-11-05 08:10:21 to 2026-11-05 09:10:21."
	for _, want := range []string{
		"Publisher user: owner-id\nAd code: code-id",
		period,
		"1 with HTTP errors, 8 with no tracked responses",
		"HTTP 200=0, 204=0, 4xx=3, 5xx=2",
		"site-0.example: HTTP errors (4xx=3, 5xx=2); last recorded request: 2026-11-05 09:00:21 UTC",
		"site-8.example: no HTTP 200, 204, 4xx or 5xx recorded",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("missing %q in %q", want, message)
		}
	}
	recovery := inventory.telegramRecoveryText("owner-id", "code-id", []string{"site-8.example"})
	for _, want := range []string{period, "All currently known zones", "Previously flagged sites:\n• site-8.example"} {
		if !strings.Contains(recovery, want) {
			t.Errorf("missing %q in %q", want, recovery)
		}
	}
}

func TestPublisherTelegramNoticeBoundsLargeInventory(t *testing.T) {
	inventory := publisherAttentionInventory{}
	zones := make([]ZoneDiagnostic, 100)
	hosts := make([]string, len(zones))
	for index := range zones {
		hosts[index] = strings.Repeat("a", 240) + ".example"
		zones[index] = ZoneDiagnostic{Hostname: hosts[index], Requests4xx: 1}
	}
	for _, message := range []string{
		inventory.telegramAttentionText("owner-id", "code-id", zones),
		inventory.telegramRecoveryText("owner-id", "code-id", hosts),
	} {
		if len(message) > 4096 || !strings.Contains(message, "not shown.") {
			t.Fatalf("expected one bounded notice with an omission count; length=%d", len(message))
		}
	}
}
