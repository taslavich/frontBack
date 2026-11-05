package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Keep the existing one-hour monitoring window. This is an inactivity check,
// independent of whether any recorded request actually returned an ad.
const publisherZoneSilenceAfter = time.Hour

type publisherSilenceNotice struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Text string    `json:"text"`
}

func publisherZoneSilenceNotice(user string, z ZoneDiagnostic, asof time.Time) (publisherSilenceNotice, bool) {
	if z.CodeID == "" || z.ZoneID == "" || z.LastRequestAt == nil || z.LastRequestAt.IsZero() {
		return publisherSilenceNotice{}, false
	}
	// The inventory timestamp covers every recorded request, including errors
	// and other statuses. Also suppress a notice if the rolling report already
	// proves traffic, even if the separately stored inventory is behind.
	age := asof.Sub(*z.LastRequestAt)
	if age < publisherZoneSilenceAfter || z.Requests200 != 0 || z.Requests204 != 0 || z.Requests4xx != 0 || z.Requests5xx != 0 {
		return publisherSilenceNotice{}, false
	}
	location := fmt.Sprintf("zone %.64s", z.ZoneID)
	if host := strings.Join(strings.Fields(z.Hostname), " "); host != "" {
		location = fmt.Sprintf("%.253s (%s)", host, location)
	}
	// Persist once per continuous silence episode, across polls and restarts.
	// A newer last request starts a new episode. Dismissal keeps the existing ID.
	key := strings.Join([]string{"publisher:zone-silence", user, z.CodeID, z.SiteID, z.ZoneID, z.LastRequestAt.UTC().Format(time.RFC3339Nano)}, ":")
	return publisherSilenceNotice{
		ID:   uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)),
		Code: z.CodeID,
		Text: fmt.Sprintf("No ad requests recorded\nWe have not recorded any ad requests from %s for %d minutes. Please check that your ad code or direct link is installed and receiving traffic.", location, age/time.Minute),
	}, true
}

func insertPublisherSilenceNotices(ctx context.Context, conn *sql.Conn, user string, notices []publisherSilenceNotice) error {
	if len(notices) == 0 {
		return nil
	}
	body, err := json.Marshal(notices)
	if err != nil {
		return err
	}
	// Recheck ownership and activation when writing. Existing IDs are retained
	// without updating even a previously dismissed notification.
	_, err = conn.ExecContext(ctx, `INSERT INTO notifications(id,user_id,status,text,type)
		SELECT n.id, c.user_id, 'active', n.text, 'other'
		FROM jsonb_to_recordset($2::jsonb) AS n(id uuid, code uuid, text text)
		JOIN ad_codes c ON c.ad_code_id=n.code AND c.user_id=$1
		WHERE c.is_active AND c.archived_at IS NULL
		ON CONFLICT (id) DO NOTHING`, user, string(body))
	return err
}
