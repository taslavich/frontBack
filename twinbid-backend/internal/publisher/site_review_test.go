package publisher

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"twinbid-backend/internal/bot"
)

type siteReviewFake struct {
	site   SiteReviewCandidate
	events []string
}

func (f *siteReviewFake) PublisherUnnotifiedSites(context.Context, int) ([]SiteReviewCandidate, error) {
	for _, event := range f.events {
		if event == "notified" || event == "allowed" || event == "blocked" || event == "block_requested" {
			return nil, nil
		}
	}
	return []SiteReviewCandidate{f.site}, nil
}
func (f *siteReviewFake) PublisherSiteAwaitingReview(_ context.Context, id string) (bool, error) {
	return id == f.site.SiteID, nil
}
func (f *siteReviewFake) PublisherSiteReviewState(context.Context, string) (SiteReviewState, error) {
	var state SiteReviewState
	for _, event := range f.events {
		switch event {
		case "allowed", "blocked":
			state.Decided = event
		case "block_requested":
			state.BlockRequested = true
		}
	}
	return state, nil
}
func (f *siteReviewFake) AppendPublisherSiteReview(_ context.Context, _, action string, _ int64) error {
	f.events = append(f.events, action)
	return nil
}

func TestPublisherSiteReviewDeliveryAndBlockRetry(t *testing.T) {
	ctx := t.Context()
	f := &siteReviewFake{site: SiteReviewCandidate{SiteID: "ABcd1234Ef56", Hostname: "example.test"}}
	sends := 0
	send := func(_ context.Context, req bot.PublisherSiteReviewRequest) error {
		if req.SiteID != f.site.SiteID || req.Hostname != f.site.Hostname {
			t.Fatalf("unexpected site: %+v", req)
		}
		sends++
		if sends == 1 {
			return errors.New("bot unavailable")
		}
		return nil
	}
	if err := dispatchSiteReviewNotices(ctx, f, send); err == nil || len(f.events) != 0 {
		t.Fatal("failed send must not mark the site notified")
	}
	for range 2 {
		if err := dispatchSiteReviewNotices(ctx, f, send); err != nil {
			t.Fatal(err)
		}
	}
	if sends != 2 || !reflect.DeepEqual(f.events, []string{"notified"}) {
		t.Fatalf("site sent again after successful delivery: %d %v", sends, f.events)
	}
	blocks := 0
	block := func() error {
		blocks++
		if blocks == 1 {
			return errors.New("policy store unavailable")
		}
		return nil
	}
	decide := func(action string) (string, error) { return applySiteReview(ctx, f, f.site.SiteID, action, 42, block) }
	if _, err := decide("blocked"); err == nil || !reflect.DeepEqual(f.events, []string{"notified", "block_requested"}) {
		t.Fatalf("failed block must keep the intent and not resolve the site: %v %v", err, f.events)
	}
	if _, err := decide("allowed"); err == nil || blocks != 1 {
		t.Fatal("Allow must not override a pending block")
	}
	if actual, err := decide("blocked"); err != nil || actual != "blocked" {
		t.Fatalf("retry failed: %q %v", actual, err)
	}
	if actual, err := decide("allowed"); err != nil || actual != "blocked" || blocks != 2 ||
		!reflect.DeepEqual(f.events, []string{"notified", "block_requested", "blocked"}) {
		t.Fatalf("replayed button changed the decision: %q %v blocks=%d events=%v", actual, err, blocks, f.events)
	}
}

func TestPublisherSiteReviewAllowDoesNotTouchBlockingPolicy(t *testing.T) {
	ctx := t.Context()
	f := &siteReviewFake{site: SiteReviewCandidate{SiteID: "ABcd1234Ef56"}}
	block := func() error { t.Fatal("Allow wrote a block policy"); return nil }
	if _, err := applySiteReview(ctx, f, "not-known-id", "allowed", 42, block); err == nil || len(f.events) != 0 {
		t.Fatal("unknown site accepted")
	}
	if actual, err := applySiteReview(ctx, f, f.site.SiteID, "allowed", 42, block); err != nil || actual != "allowed" {
		t.Fatalf("Allow failed: %q %v", actual, err)
	}
}
