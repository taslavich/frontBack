package tronscan

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyUSDTTransfer(t *testing.T) {
	const wallet = "TJr26CGefYQAQ5ryrxETrQPeLYdzmz52ad"
	const contract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

	tests := []struct {
		name       string
		body       string
		expected   int64
		wantState  VerificationState
		wantReason string
	}{
		{
			name:      "verified exact amount",
			body:      fmt.Sprintf(`{"hash":"abc","confirmed":true,"revert":false,"confirmations":4,"contractRet":"SUCCESS","trc20TransferInfo":[{"to_address":%q,"contract_address":%q,"amount_str":"100000000","decimals":6,"type":"Transfer","status":0,"tokenType":"trc20"}]}`, wallet, contract),
			expected:  100000000,
			wantState: VerificationVerified,
		},
		{
			name:      "pending confirmation",
			body:      `{"hash":"abc","confirmed":false,"revert":false,"confirmations":0,"contractRet":"SUCCESS"}`,
			expected:  100000000,
			wantState: VerificationPending,
		},
		{
			name:       "wrong amount",
			body:       fmt.Sprintf(`{"hash":"abc","confirmed":true,"revert":false,"confirmations":4,"contractRet":"SUCCESS","trc20TransferInfo":[{"to_address":%q,"contract_address":%q,"amount_str":"99000000","decimals":6,"type":"Transfer","status":0,"tokenType":"trc20"}]}`, wallet, contract),
			expected:   100000000,
			wantState:  VerificationInvalid,
			wantReason: "USDT amount mismatch",
		},
		{
			name:       "wrong wallet",
			body:       fmt.Sprintf(`{"hash":"abc","confirmed":true,"revert":false,"confirmations":4,"contractRet":"SUCCESS","trc20TransferInfo":[{"to_address":"TWrongWallet","contract_address":%q,"amount_str":"100000000","decimals":6,"type":"Transfer","status":0,"tokenType":"trc20"}]}`, contract),
			expected:   100000000,
			wantState:  VerificationInvalid,
			wantReason: "configured wallet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("TRON-PRO-API-KEY"); got != "secret" {
					t.Fatalf("API key header=%q", got)
				}
				if got := r.URL.Query().Get("hash"); got != "abc" {
					t.Fatalf("hash=%q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(Config{
				BaseURL:          server.URL,
				APIKey:           "secret",
				WalletAddress:    wallet,
				USDTContract:     contract,
				MinConfirmations: 1,
			})
			verification, err := client.VerifyUSDTTransfer(context.Background(), "abc", big.NewInt(tt.expected))
			if err != nil {
				t.Fatal(err)
			}
			if verification.State != tt.wantState {
				t.Fatalf("state=%q want=%q reason=%q", verification.State, tt.wantState, verification.Reason)
			}
			if tt.wantReason != "" && !strings.Contains(verification.Reason, tt.wantReason) {
				t.Fatalf("reason=%q does not contain %q", verification.Reason, tt.wantReason)
			}
		})
	}
}
