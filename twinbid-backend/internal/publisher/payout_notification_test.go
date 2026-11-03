package publisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"twinbid-backend/internal/bot"
)

func TestNotifyPayoutRoutesOnlyPendingToModeration(t *testing.T) {
	payout := bot.PublisherPayoutModerationRequest{
		ID: "b29dbfc7-d5fb-4b3d-aa90-d80158ec4283", UserID: "publisher", Amount: "75.12345678", Method: "usdt_erc20", Wallet: "wallet",
	}
	type request struct {
		path, method, secret string
		body                 map[string]string
	}
	calls := make(chan request, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		calls <- request{r.URL.Path, r.Method, r.Header.Get("X-Bot-Secret"), body}
		if body["wallet"] == "fail" {
			http.Error(w, "Telegram unavailable", http.StatusBadGateway)
		}
	}))
	defer server.Close()
	s := Service{Bot: bot.NewBotClient(server.URL, "test-secret")}
	for _, status := range []string{"pending", "paid", "canceled"} {
		t.Run(status, func(t *testing.T) {
			if err := s.notifyPayout(context.Background(), status, payout); err != nil {
				t.Fatal(err)
			}
			got := <-calls
			if got.method != http.MethodPost || got.secret != "test-secret" {
				t.Fatalf("wrong authentication or method: %#v", got)
			}
			if status == "pending" {
				if got.path != "/internal/publisher/payouts/moderation" ||
					got.body["id"] != payout.ID || got.body["user_id"] != payout.UserID || got.body["amount"] != payout.Amount ||
					got.body["method"] != payout.Method || got.body["wallet"] != payout.Wallet {
					t.Fatalf("wrong pending payout: %#v", got)
				}
			} else if got.path != "/internal/messages/send" || !strings.Contains(got.body["text"], payout.ID+": "+status) {
				t.Fatalf("wrong final status notification: %#v", got)
			}
		})
	}
	payout.Wallet = "fail"
	if err := s.notifyPayout(context.Background(), "pending", payout); err == nil {
		t.Fatal("delivery failure must be returned so Notify does not mark the payout as notified")
	}
	if len(calls) != 1 || (<-calls).path != "/internal/publisher/payouts/moderation" {
		t.Fatal("failed moderation must not fall back to the system chat")
	}
}
