package publisher

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func TestPublisherZoneSilenceNotice(t *testing.T) {
	now := time.Date(2026, 11, 5, 9, 10, 0, 0, time.UTC)
	previous := now.Add(-75 * time.Minute)
	silent := ZoneDiagnostic{CodeID: "code", SiteID: "site", ZoneID: "zone-a", Hostname: "example.com", LastRequestAt: &previous, Status: "problem"}
	notice, ok := publisherZoneSilenceNotice("owner", silent, now)
	want := "No ad requests recorded\nWe have not recorded any ad requests from example.com (zone zone-a) for 75 minutes. Please check that your ad code or direct link is installed and receiving traffic."
	if !ok || notice.Text != want {
		t.Fatalf("unexpected notice: %+v, %t", notice, ok)
	}
	if next, _ := publisherZoneSilenceNotice("owner", silent, now.Add(time.Minute)); next.ID != notice.ID {
		t.Fatal("polling during a single outage must reuse the notice ID")
	}
	silent.ZoneID = "zone-b"
	if next, _ := publisherZoneSilenceNotice("owner", silent, now); next.ID == notice.ID {
		t.Fatal("two zones must have separate notices")
	}
	silent.ZoneID = "zone-a"
	afterResume := now.Add(2 * time.Minute)
	silent.LastRequestAt = &afterResume
	if next, ok := publisherZoneSilenceNotice("owner", silent, afterResume.Add(time.Hour)); !ok || next.ID == notice.ID {
		t.Fatal("traffic resuming must allow a new notice after another hour")
	}

	recent := now.Add(-time.Minute)
	short := now.Add(-time.Hour + time.Second)
	future := now.Add(time.Minute)
	zero := time.Time{}
	for _, tc := range []struct {
		name                            string
		last                            *time.Time
		success, noFill, denied, failed uint64
	}{
		{name: "no recorded activity ever"},
		{name: "zero timestamp", last: &zero},
		{name: "clock skew", last: &future},
		{name: "before threshold", last: &short},
		{name: "recent request with no classified status", last: &recent},
		{name: "normal response", last: &previous, success: 1},
		{name: "normal response without ad", last: &previous, noFill: 1},
		{name: "IP restriction or other 4xx", last: &previous, denied: 1},
		{name: "timeout or other 5xx", last: &previous, failed: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			z := silent
			z.LastRequestAt = tc.last
			z.Requests200, z.Requests204, z.Requests4xx, z.Requests5xx = tc.success, tc.noFill, tc.denied, tc.failed
			if n, ok := publisherZoneSilenceNotice("owner", z, now); ok {
				t.Fatalf("unexpected notice: %q", n.Text)
			}
		})
	}
}

func TestPublisherSilenceNoticePostgres(t *testing.T) {
	dsn := os.Getenv("PUBLISHER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("PUBLISHER_TEST_POSTGRES_DSN unset")
	}
	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TEMP TABLE notifications(id uuid PRIMARY KEY,user_id uuid,status text,text text,type text)`)
	exec(`CREATE TEMP TABLE ad_codes(ad_code_id uuid PRIMARY KEY,user_id uuid,is_active boolean,archived_at timestamptz)`)
	owner, other := uuid.NewString(), uuid.NewString()
	code, foreign, paused, archived := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO ad_codes VALUES ($1,$2,true,NULL),($3,$4,true,NULL),($5,$2,false,NULL),($6,$2,true,now())`, code, owner, foreign, other, paused, archived)
	now := time.Now().UTC()
	last := now.Add(-time.Hour)
	notice := func(code string, last time.Time, asof time.Time) publisherSilenceNotice {
		n, ok := publisherZoneSilenceNotice(owner, ZoneDiagnostic{CodeID: code, SiteID: "site", ZoneID: "zone", LastRequestAt: &last}, asof)
		if !ok {
			t.Fatal("expected a notice candidate")
		}
		return n
	}
	original := notice(code, last, now)
	insert := func(notices ...publisherSilenceNotice) {
		t.Helper()
		if err := insertPublisherSilenceNotices(ctx, conn, owner, notices); err != nil {
			t.Fatal(err)
		}
	}
	insert(original, notice(foreign, last, now), notice(paused, last, now), notice(archived, last, now))
	exec(`UPDATE notifications SET status='dismissed' WHERE id=$1`, original.ID)
	insert(original, notice(code, now, now.Add(time.Hour)))
	var total, dismissed, active int
	if err := conn.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER (WHERE status='dismissed'),count(*) FILTER (WHERE status='active' AND user_id=$1) FROM notifications`, owner).Scan(&total, &dismissed, &active); err != nil || total != 2 || dismissed != 1 || active != 1 {
		t.Fatalf("wrong ownership, dedup or episode counts: %d %d %d (%v)", total, dismissed, active, err)
	}
}
