package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const (
	ProviderPassimPay = "passimpay"
	ProviderCryptomus = "cryptomus"

	ChannelPassimPayInvoice = "passimpay_invoice"
	ChannelCryptomusInvoice = "cryptomus_invoice"
)

type CreateInvoiceRequest struct {
	OrderID  string
	Amount   float64
	Currency string
	Lifetime time.Duration
}

type CreateInvoiceResult struct {
	PaymentURL            string
	ProviderPaymentID     string
	ProviderTransactionID string
	ProviderStatus        string
	Raw                   json.RawMessage
}

type InvoiceStatus struct {
	PaymentURL            string
	Status                string
	ProviderPaymentID     string
	ProviderTransactionID string
	TransactionHash       string
	AmountPaid            *float64
	AmountCredited        *float64
	FeeService            *float64
	FeeNetwork            *float64
	Raw                   json.RawMessage
}

type WebhookEvent struct {
	OrderID   string
	Signature string
	Status    InvoiceStatus
}

// InvoiceProvider isolates provider-specific API/signature/payload details from
// the top-up business flow. Routes remain provider-specific so nginx can apply
// an independent IP allowlist to every webhook URL.
type InvoiceProvider interface {
	Name() string
	PaymentChannel() string
	Enabled() bool
	CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (CreateInvoiceResult, error)
	CheckInvoice(ctx context.Context, orderID string) (InvoiceStatus, error)
	ParseAndVerifyWebhook(raw []byte, headers http.Header) (WebhookEvent, error)
}
