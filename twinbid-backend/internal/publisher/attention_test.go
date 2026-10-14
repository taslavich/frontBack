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

func (p *inventoryPages) PublisherInventory(ctx context.Context, user, group, site, code string, limit, offset int) ([]Row, error) {
	if user != "owner" || limit != 500 {
		panic("invalid owner/page")
	}
	p.seen = append(p.seen, offset)
	if offset == p.failOffset {
		return nil, errors.New("CH outage")
	}
	out := []Row{}
	old := time.Now().Add(-10 * time.Minute)
	for i := offset; i < p.count && i < offset+limit; i++ {
		out = append(out, Row{CodeID: "code", Hostname: fmt.Sprint(i), LastRequestAt: &old})
	}
	return out, nil
}
func TestAttentionPages(t *testing.T) {
	for _, n := range []int{0, 499, 500, 501, 1501} {
		p := &inventoryPages{count: n, failOffset: -1}
		known, missing, e := attentionInventory(context.Background(), p, "owner")
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
	if _, _, e := attentionInventory(context.Background(), p, "owner"); e == nil {
		t.Fatal("partial outage accepted")
	}
}
