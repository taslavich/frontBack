package cryptomus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"twinbid-backend/internal/payments"
)

func TestCreateInvoiceSignsRequest(t *testing.T) {
	const (
		merchant = "merchant-uuid"
		apiKey   = "payment-api-key"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/payment" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Header.Get("merchant"); got != merchant {
			t.Fatalf("merchant header=%q, want %q", got, merchant)
		}
		if got, want := r.Header.Get("sign"), requestSignature(body, apiKey); got != want {
			t.Fatalf("sign header=%q, want %q", got, want)
		}

		var payload struct {
			Amount      string `json:"amount"`
			Currency    string `json:"currency"`
			OrderID     string `json:"order_id"`
			URLCallback string `json:"url_callback"`
			Subtract    int    `json:"subtract"`
			Lifetime    int    `json:"lifetime"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Amount != "10.25" || payload.Currency != "USD" || payload.OrderID != "order-1" {
			t.Fatalf("unexpected invoice payload: %#v", payload)
		}
		if payload.URLCallback != "https://api.example/api/webhooks/cryptomus" || payload.Subtract != 100 || payload.Lifetime != 3600 {
			t.Fatalf("unexpected Cryptomus options: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":0,"result":{"uuid":"payment-uuid","order_id":"order-1","payment_status":"check","url":"https://pay.example/invoice-1","is_final":false}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:         server.URL,
		MerchantUUID:    merchant,
		PaymentAPIKey:   apiKey,
		WebhookURL:      "https://api.example/api/webhooks/cryptomus",
		SubtractPercent: 100,
		Timeout:         time.Second,
	})
	result, err := client.CreateInvoice(context.Background(), payments.CreateInvoiceRequest{
		OrderID:  "order-1",
		Amount:   10.25,
		Currency: "USD",
		Lifetime: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentURL != "https://pay.example/invoice-1" || result.ProviderPaymentID != "payment-uuid" || result.ProviderStatus != "waiting" {
		t.Fatalf("unexpected create result: %#v", result)
	}
}

func TestCheckInvoiceByOrderID(t *testing.T) {
	const (
		merchant = "merchant-uuid"
		apiKey   = "payment-api-key"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/payment/info" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := r.Header.Get("sign"), requestSignature(body, apiKey); got != want {
			t.Fatalf("sign header=%q, want %q", got, want)
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["order_id"] != "order-1" {
			t.Fatalf("unexpected status payload: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":0,"result":{"uuid":"payment-uuid","order_id":"order-1","payment_amount":"10.25","merchant_amount":"10.00","payment_status":"paid_over","status":"paid_over","is_final":true,"txid":"0xabc"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:       server.URL,
		MerchantUUID:  merchant,
		PaymentAPIKey: apiKey,
		WebhookURL:    "https://api.example/api/webhooks/cryptomus",
		Timeout:       time.Second,
	})
	status, err := client.CheckInvoice(context.Background(), "order-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "paid" || status.ProviderPaymentID != "payment-uuid" || status.TransactionHash != "0xabc" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.AmountPaid == nil || *status.AmountPaid != 10.25 {
		t.Fatalf("unexpected paid amount: %#v", status.AmountPaid)
	}
}

func TestParseAndVerifyWebhook(t *testing.T) {
	const apiKey = "payment-api-key"
	client := NewClient(Config{
		BaseURL:       "https://api.cryptomus.com",
		MerchantUUID:  "merchant-uuid",
		PaymentAPIKey: apiKey,
		WebhookURL:    "https://api.example/api/webhooks/cryptomus",
	})

	// Cryptomus documents that PHP signs a representation with escaped slashes.
	// Simulate a webhook delivered with an unescaped slash while the signature is
	// based on the escaped representation; the verifier supports both forms.
	unsigned := []byte(`{"type":"payment","uuid":"payment-uuid","order_id":"order-1","payment_amount":"10.00","merchant_amount":"9.90","commission":"0.10","is_final":true,"status":"paid","convert":{"to_currency":"USDT","rate":"1"},"txid":"someTxid/WithSlash","transfer_id":"transfer-1"}`)
	sign := requestSignature(escapeUnescapedSlashes(unsigned), apiKey)
	signed := append([]byte(nil), unsigned[:len(unsigned)-1]...)
	signed = append(signed, []byte(`,"sign":"`+sign+`"}`)...)

	event, err := client.ParseAndVerifyWebhook(signed, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if event.OrderID != "order-1" || event.Status.Status != "paid" || event.Status.ProviderPaymentID != "payment-uuid" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Status.TransactionHash != "someTxid/WithSlash" || event.Status.ProviderTransactionID != "transfer-1" {
		t.Fatalf("unexpected webhook transaction fields: %#v", event.Status)
	}

	bad := append([]byte(nil), signed...)
	bad[len(bad)-3] = '0'
	if _, err := client.ParseAndVerifyWebhook(bad, http.Header{}); err == nil {
		t.Fatal("expected invalid webhook signature")
	}
}

func TestNormalizePaymentStatus(t *testing.T) {
	trueValue := true
	falseValue := false
	cases := []struct {
		status  string
		isFinal *bool
		want    string
	}{
		{status: "paid", isFinal: &trueValue, want: "paid"},
		{status: "paid_over", isFinal: &trueValue, want: "paid"},
		{status: "paid", isFinal: &falseValue, want: "waiting"},
		{status: "paid", isFinal: nil, want: "waiting"},
		{status: "confirm_check", isFinal: &falseValue, want: "waiting"},
		{status: "wrong_amount_waiting", isFinal: &falseValue, want: "waiting"},
		{status: "wrong_amount", isFinal: &trueValue, want: "error"},
		{status: "cancel", isFinal: &trueValue, want: "error"},
		{status: "locked", isFinal: &falseValue, want: "waiting"},
		{status: "future_status", isFinal: &falseValue, want: "waiting"},
	}
	for _, tt := range cases {
		if got := normalizePaymentStatus(tt.status, tt.isFinal); got != tt.want {
			t.Fatalf("normalizePaymentStatus(%q)=%q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestRequestSignatureKnownVector(t *testing.T) {
	body := []byte(`{"amount":"15","currency":"USD","order_id":"1"}`)
	const want = "ea14703a061683fdda5e6577b9bc5450"
	if got := requestSignature(body, "secret"); got != want {
		t.Fatalf("requestSignature=%q, want %q", got, want)
	}
}

func TestWebhookSignatureFieldMayAppearInMiddle(t *testing.T) {
	const apiKey = "payment-api-key"
	unsigned := []byte(`{"type":"payment","order_id":"order-1","status":"paid","is_final":true}`)
	sign := requestSignature(unsigned, apiKey)
	signed := []byte(`{"type":"payment","sign":"` + sign + `","order_id":"order-1","status":"paid","is_final":true}`)

	client := NewClient(Config{
		BaseURL:       "https://api.cryptomus.com",
		MerchantUUID:  "merchant-uuid",
		PaymentAPIKey: apiKey,
		WebhookURL:    "https://api.example/api/webhooks/cryptomus",
	})
	if _, err := client.ParseAndVerifyWebhook(signed, http.Header{}); err != nil {
		t.Fatal(err)
	}
}
