package publisher

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type publisherAttentionInventory struct {
	windowFrom, windowTo time.Time
	silenceNotices       []publisherSilenceNotice
	// A present code with an empty slice has known zones, none marked as a problem.
	problemsByCode map[string][]ZoneDiagnostic
}

// Consume bounded pages before acting on attention/recovery. An incomplete
// page sequence is an error, never evidence that all sites recovered.
func attentionInventory(ctx context.Context, provider ZoneReporter, user string, codes []string) (publisherAttentionInventory, error) {
	result := publisherAttentionInventory{problemsByCode: make(map[string][]ZoneDiagnostic)}
	asof := time.Now().UTC()
	for offset := 0; ; offset += 500 {
		if e := ctx.Err(); e != nil {
			return publisherAttentionInventory{}, e
		}
		report, e := provider.PublisherZoneDiagnostics(ctx, user, codes, 500, offset, asof)
		if e != nil {
			return publisherAttentionInventory{}, e
		}
		if offset == 0 {
			result.windowFrom, result.windowTo = report.WindowFrom, report.WindowTo
		}
		for _, z := range report.Rows {
			if n, ok := publisherZoneSilenceNotice(user, z, asof); ok {
				result.silenceNotices = append(result.silenceNotices, n)
			}
			if z.Status == "problem" {
				result.problemsByCode[z.CodeID] = append(result.problemsByCode[z.CodeID], z)
			} else if _, known := result.problemsByCode[z.CodeID]; !known {
				result.problemsByCode[z.CodeID] = nil
			}
		}
		if len(report.Rows) < 500 {
			break
		}
	}
	return result, nil
}

const noticeDateTime = "2006-01-02 15:04:05"

func (i publisherAttentionInventory) periodText() string {
	return fmt.Sprintf("Checked period (UTC): %s to %s.", i.windowFrom.UTC().Format(noticeDateTime), i.windowTo.UTC().Format(noticeDateTime))
}

func noticeHostname(host string) string {
	host = strings.Join(strings.Fields(host), " ")
	if host == "" {
		return "Hostname unavailable"
	}
	return fmt.Sprintf("%.253s", host)
}

func (i publisherAttentionInventory) telegramAttentionText(user, code string, problems []ZoneDiagnostic) string {
	var n4xx, n5xx uint64
	var withErrors int
	for _, z := range problems {
		n4xx += z.Requests4xx
		n5xx += z.Requests5xx
		if z.Requests4xx > 0 || z.Requests5xx > 0 {
			withErrors++
		}
	}
	var message strings.Builder
	fmt.Fprintf(&message, "Publisher attention: no HTTP 200 or 204 in %d zone(s)\nPublisher user: %s\nAd code: %s\n%s\nAffected zones: %d with HTTP errors, %d with no tracked responses. Totals: HTTP 200=0, 204=0, 4xx=%d, 5xx=%d.\n200 = ad returned; 204 = normal response without an ad. No tracked responses can mean no traffic was recorded.", len(problems), user, code, i.periodText(), withErrors, len(problems)-withErrors, n4xx, n5xx)
	for index, z := range problems {
		var line strings.Builder
		fmt.Fprintf(&line, "• %s", noticeHostname(z.Hostname))
		if z.ZoneID != "" {
			fmt.Fprintf(&line, " (zone %.64s)", z.ZoneID)
		}
		if z.Requests4xx == 0 && z.Requests5xx == 0 {
			line.WriteString(": no HTTP 200, 204, 4xx or 5xx recorded")
		} else {
			fmt.Fprintf(&line, ": HTTP errors (4xx=%d, 5xx=%d)", z.Requests4xx, z.Requests5xx)
		}
		if z.LastRequestAt != nil && !z.LastRequestAt.After(i.windowTo) {
			fmt.Fprintf(&line, "; last recorded request: %s UTC", z.LastRequestAt.UTC().Format(noticeDateTime))
		}
		if !appendTelegramNoticeLine(&message, line.String()) {
			fmt.Fprintf(&message, "\n%d more affected zone(s) not shown.", len(problems)-index)
			break
		}
	}
	return message.String()
}

func (i publisherAttentionInventory) telegramRecoveryText(user, code string, previousHosts []string) string {
	var message strings.Builder
	fmt.Fprintf(&message, "Publisher recovery: normal responses recorded again\nPublisher user: %s\nAd code: %s\n%s\nAll currently known zones recorded at least one HTTP 200 (ad returned) or HTTP 204 (no ad).", user, code, i.periodText())
	if len(previousHosts) > 0 {
		message.WriteString("\nPreviously flagged sites:")
		for index, host := range previousHosts {
			if !appendTelegramNoticeLine(&message, "• "+noticeHostname(host)) {
				fmt.Fprintf(&message, "\n%d more previous entries not shown.", len(previousHosts)-index)
				break
			}
		}
	}
	return message.String()
}

func appendTelegramNoticeLine(message *strings.Builder, line string) bool {
	// Leave room for the omitted-count line and stay within one Telegram message.
	const detailsLimit = 3500
	if message.Len()+1+len(line) > detailsLimit {
		return false
	}
	message.WriteByte('\n')
	message.WriteString(line)
	return true
}
