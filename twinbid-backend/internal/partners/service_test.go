package partners

import (
	"context"
	"testing"
)

type fakeStatsRepository struct {
	gotUserID string
	out       Stats
	err       error
}

func (f *fakeStatsRepository) Stats(_ context.Context, userID string) (Stats, error) {
	f.gotUserID = userID
	return f.out, f.err
}

func TestServiceStats(t *testing.T) {
	repo := &fakeStatsRepository{out: Stats{
		Partner:     "TB7K2M9X4QF8",
		Advertisers: 3,
		Turnover:    1250.50,
		Withdrawn:   75,
	}}
	svc := NewService(repo)

	got, err := svc.Stats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Stats returned error: %v", err)
	}
	if repo.gotUserID != "user-1" {
		t.Fatalf("repository got user id %q, want %q", repo.gotUserID, "user-1")
	}
	if got != repo.out {
		t.Fatalf("Stats = %#v, want %#v", got, repo.out)
	}
}
