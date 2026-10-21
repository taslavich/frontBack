package publisher

import (
	"context"
	"errors"
	"fmt"
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
	return ZoneDiagnostics{Rows: out}, nil
}
func TestAttentionPages(t *testing.T) {
	for _, n := range []int{0, 499, 500, 501, 1501} {
		p := &inventoryPages{count: n, failOffset: -1}
		known, missing, e := attentionInventory(context.Background(), p, "owner", []string{"code"})
		if e != nil || len(missing["code"]) != n {
			t.Fatal(n, len(missing["code"]), e)
		}
		if n > 0 && !known["code"] {
			t.Fatal("lost code")
		}
		want := n/500 + 1
		if len(p.seen) != want {
			t.Fatal(n, p.seen)
		}
	}
	p := &inventoryPages{count: 1000, failOffset: 500}
	if _, _, e := attentionInventory(context.Background(), p, "owner", []string{"code"}); e == nil {
		t.Fatal("partial outage accepted")
	}
}
