package stats

import (
	"context"
	"fmt"
	"twinbid-backend/internal/publisher"
)

func (s *Service) publisherSiteReviewTable(name string) (*ClickHouseRepository, string, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return nil, "", fmt.Errorf("publisher site review requires ClickHouse")
	}
	table, err := publisherCompanionTable(r.table, name)
	return r, table, err
}

func (s *Service) PublisherUnnotifiedSites(ctx context.Context, limit int) ([]publisher.SiteReviewCandidate, error) {
	r, view, err := s.publisherSiteReviewTable("publisher_new_sites")
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid site review batch size")
	}
	rows, err := r.db.QueryContext(ctx, "SELECT site_id, hostname, first_request_at FROM "+view+
		" WHERE notified=0 AND block_requested=0 ORDER BY first_request_at, site_id LIMIT ? SETTINGS max_threads=2", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sites := []publisher.SiteReviewCandidate{}
	for rows.Next() {
		var site publisher.SiteReviewCandidate
		if err := rows.Scan(&site.SiteID, &site.Hostname, &site.FirstRequestAt); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (s *Service) PublisherSiteAwaitingReview(ctx context.Context, siteID string) (bool, error) {
	r, view, err := s.publisherSiteReviewTable("publisher_new_sites")
	if err != nil {
		return false, err
	}
	var n uint64
	err = r.db.QueryRowContext(ctx, "SELECT count() FROM "+view+" WHERE site_id=?", siteID).Scan(&n)
	return n > 0, err
}

func (s *Service) PublisherSiteReviewState(ctx context.Context, siteID string) (publisher.SiteReviewState, error) {
	r, table, err := s.publisherSiteReviewTable("publisher_site_review")
	if err != nil {
		return publisher.SiteReviewState{}, err
	}
	var blocked, allowed, requested uint64
	err = r.db.QueryRowContext(ctx, "SELECT countIf(action='blocked'),countIf(action='allowed'),countIf(action='block_requested') FROM "+
		table+" WHERE site_id=?", siteID).Scan(&blocked, &allowed, &requested)
	out := publisher.SiteReviewState{BlockRequested: requested > 0}
	if blocked > 0 {
		out.Decided = "blocked"
	} else if allowed > 0 {
		out.Decided = "allowed"
	}
	return out, err
}

func (s *Service) AppendPublisherSiteReview(ctx context.Context, siteID, action string, moderator int64) error {
	r, table, err := s.publisherSiteReviewTable("publisher_site_review")
	if err != nil {
		return err
	}
	// Force a synchronous insert even if the ClickHouse user enables async
	// inserts, so the next decision or polling cycle can see this event.
	_, err = r.db.ExecContext(ctx, "INSERT INTO "+table+
		" (site_id, action, moderator_id) SETTINGS async_insert=0 VALUES (?, ?, ?)", siteID, action, moderator)
	return err
}
