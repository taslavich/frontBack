package topups

import (
	"context"
	"net/http"
	"testing"

	"twinbid-backend/internal/config"
	"twinbid-backend/internal/payments"
)

func TestNormalizeMoney(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		want    float64
		wantErr bool
	}{
		{name: "minimum", value: 100, want: 100},
		{name: "two decimals", value: 100.25, want: 100.25},
		{name: "floating representation", value: 100.1 + 0.2, want: 100.3},
		{name: "below minimum", value: 99.99, wantErr: true},
		{name: "zero", value: 0, wantErr: true},
		{name: "negative", value: -1, wantErr: true},
		{name: "too many decimals", value: 100.251, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMoney(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeMoney(%v) unexpectedly succeeded: %v", tt.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeMoney(%v): %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeMoney(%v)=%v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestRoundMoneyRoundsPromoCreditToCents(t *testing.T) {
	if got, want := roundMoney(10*(1+12.5/100)), 11.25; got != want {
		t.Fatalf("roundMoney promo result=%v, want %v", got, want)
	}
}

type fakeInvoiceProvider struct {
	name    string
	channel string
}

func (p fakeInvoiceProvider) Name() string           { return p.name }
func (p fakeInvoiceProvider) PaymentChannel() string { return p.channel }
func (p fakeInvoiceProvider) Enabled() bool          { return true }
func (p fakeInvoiceProvider) CreateInvoice(context.Context, payments.CreateInvoiceRequest) (payments.CreateInvoiceResult, error) {
	return payments.CreateInvoiceResult{}, nil
}
func (p fakeInvoiceProvider) CheckInvoice(context.Context, string) (payments.InvoiceStatus, error) {
	return payments.InvoiceStatus{}, nil
}
func (p fakeInvoiceProvider) ParseAndVerifyWebhook([]byte, http.Header) (payments.WebhookEvent, error) {
	return payments.WebhookEvent{}, nil
}

func TestResolvePaymentSelectionRequiresProviderForInvoice(t *testing.T) {
	svc := NewService(
		nil, nil, nil, nil, nil, config.BotConfig{},
		fakeInvoiceProvider{name: payments.ProviderPassimPay, channel: PaymentChannelPassimPayInvoice},
		fakeInvoiceProvider{name: payments.ProviderCryptomus, channel: PaymentChannelCryptomusInvoice},
	)

	if _, _, err := svc.resolvePaymentSelection(CreateTopupRequest{}); err == nil {
		t.Fatal("expected missing provider to fail")
	}

	channel, provider, err := svc.resolvePaymentSelection(CreateTopupRequest{Provider: payments.ProviderPassimPay})
	if err != nil {
		t.Fatal(err)
	}
	if channel != PaymentChannelPassimPayInvoice || provider == nil || provider.Name() != payments.ProviderPassimPay {
		t.Fatalf("unexpected PassimPay selection: channel=%q provider=%v", channel, provider)
	}

	channel, provider, err = svc.resolvePaymentSelection(CreateTopupRequest{Provider: payments.ProviderCryptomus})
	if err != nil {
		t.Fatal(err)
	}
	if channel != PaymentChannelCryptomusInvoice || provider == nil || provider.Name() != payments.ProviderCryptomus {
		t.Fatalf("unexpected Cryptomus selection: channel=%q provider=%v", channel, provider)
	}

	if _, _, err := svc.resolvePaymentSelection(CreateTopupRequest{Provider: "unknown"}); err == nil {
		t.Fatal("expected unsupported provider to fail")
	}
}

func TestResolvePaymentSelectionKeepsExplicitStaticWallet(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, config.BotConfig{})
	channel, provider, err := svc.resolvePaymentSelection(CreateTopupRequest{PaymentChannel: PaymentChannelStaticWallet})
	if err != nil {
		t.Fatal(err)
	}
	if channel != PaymentChannelStaticWallet || provider != nil {
		t.Fatalf("unexpected static-wallet selection: channel=%q provider=%v", channel, provider)
	}
}

func TestUSDTMicroAmount(t *testing.T) {
	got, err := usdtMicroAmount(100.25)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "100250000" {
		t.Fatalf("micro amount=%s want=100250000", got.String())
	}
}

func TestIsUSDTTRC20PaymentMethodAcceptsStoredAndLegacyForms(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "usdt_trc20", want: true},
		{value: "USDT_TRC20", want: true},
		{value: "USDT TRC20", want: true},
		{value: "  usdt trc20  ", want: true},
		{value: "usdt-trc20", want: true},
		{value: "Usdt Trc20", want: true},
		{value: "usdt_erc20", want: false},
		{value: "", want: false},
	}

	for _, tt := range tests {
		if got := isUSDTTRC20PaymentMethod(tt.value); got != tt.want {
			t.Fatalf("isUSDTTRC20PaymentMethod(%q)=%t, want %t", tt.value, got, tt.want)
		}
	}
}
