package passimpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"twinbid-backend/internal/payments"
)

func TestSignatureIsStableForCanonicalJSON(t *testing.T) {
	payload := map[string]any{"platformId": int64(123), "orderId": "order-1", "amount": "10.00"}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	first := signature(123, body, "secret")
	second := signature(123, body, "secret")
	if first == "" || first != second {
		t.Fatalf("unexpected signature: %q vs %q", first, second)
	}
}

func TestCreateInvoiceSignsRequestAndAcceptsUnsignedResponse(t *testing.T) {
	const (
		platformID = int64(123)
		apiKey     = "secret"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/createorder" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := r.Header.Get("x-signature"), signature(platformID, canonical, apiKey); got != want {
			t.Fatalf("request signature=%q, want %q", got, want)
		}
		if payload["orderId"] != "order-1" || payload["amount"] != "10.10" || payload["symbol"] != "USD" {
			t.Fatalf("unexpected payload: %#v", payload)
		}

		responseBody, _ := json.Marshal(map[string]any{
			"result": 1,
			"url":    "https://pay.example/invoice-1",
			"status": "wait",
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		PlatformID: platformID,
		APIKey:     apiKey,
		Timeout:    time.Second,
	})
	result, err := client.CreateInvoice(context.Background(), payments.CreateInvoiceRequest{OrderID: "order-1", Amount: 10, Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentURL != "https://pay.example/invoice-1" || result.ProviderStatus != "waiting" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCheckInvoiceUsesV2PathAndAcceptsUnsignedResponse(t *testing.T) {
	const (
		platformID = int64(123)
		apiKey     = "secret"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/orderstatus" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := r.Header.Get("x-signature"), signature(platformID, canonical, apiKey); got != want {
			t.Fatalf("request signature=%q, want %q", got, want)
		}
		if payload["orderId"] != "order-1" {
			t.Fatalf("unexpected payload: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":1,"status":"paid","paymentId":"payment-1"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:    server.URL,
		PlatformID: platformID,
		APIKey:     apiKey,
		Timeout:    time.Second,
	})
	status, err := client.CheckInvoice(context.Background(), "order-1")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "paid" || status.ProviderPaymentID != "payment-1" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestVerifyAndParseWebhook(t *testing.T) {
	client := NewClient(Config{
		BaseURL:    "https://api.example",
		PlatformID: 123,
		APIKey:     "secret",
	})
	body, _ := json.Marshal(map[string]any{
		"type":          "deposit",
		"platformId":    123,
		"paymentId":     77,
		"orderId":       "order-1",
		"amount":        "10.0",
		"amountReceive": "9.8",
		"txhash":        "0xabc",
	})
	if err := client.VerifyPayload(body, signature(123, body, "secret")); err != nil {
		t.Fatal(err)
	}
	status, orderID, err := client.ParseWebhook(body)
	if err != nil {
		t.Fatal(err)
	}
	if orderID != "order-1" || status.Status != "waiting" || status.TransactionHash != "0xabc" {
		t.Fatalf("unexpected webhook result: order=%q status=%#v", orderID, status)
	}
	if status.AmountCredited == nil || *status.AmountCredited != 9.8 {
		t.Fatalf("unexpected credited amount: %#v", status.AmountCredited)
	}
	if err := client.VerifyPayload(body, "invalid"); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"paid":    "paid",
		"wait":    "waiting",
		"pending": "waiting",
		"error":   "error",
	}
	for input, want := range cases {
		if got := normalizeStatus(input); got != want {
			t.Fatalf("normalizeStatus(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestInvoiceAmountPreservesHistoricalOnePercentFee(t *testing.T) {
	cases := []struct {
		deposit float64
		want    float64
	}{
		{deposit: 100, want: 101},
		{deposit: 10, want: 10.10},
		{deposit: 10.25, want: 10.35},
	}

	for _, tt := range cases {
		if got := invoiceAmount(tt.deposit); got != tt.want {
			t.Fatalf("invoiceAmount(%v)=%v, want %v", tt.deposit, got, tt.want)
		}
	}
}
