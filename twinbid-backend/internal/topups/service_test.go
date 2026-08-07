package topups

import "testing"

func TestNormalizeMoney(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		want    float64
		wantErr bool
	}{
		{name: "integer", value: 10, want: 10},
		{name: "two decimals", value: 10.25, want: 10.25},
		{name: "floating representation", value: 0.1 + 0.2, want: 0.3},
		{name: "zero", value: 0, wantErr: true},
		{name: "negative", value: -1, wantErr: true},
		{name: "too many decimals", value: 10.251, wantErr: true},
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

func TestPassimPayInvoiceAmountAddsOnePercent(t *testing.T) {
	tests := []struct {
		deposit float64
		want    float64
	}{
		{deposit: 100, want: 101},
		{deposit: 10, want: 10.10},
		{deposit: 10.25, want: 10.35},
	}

	for _, tt := range tests {
		if got := passimPayInvoiceAmount(tt.deposit); got != tt.want {
			t.Fatalf("passimPayInvoiceAmount(%v)=%v, want %v", tt.deposit, got, tt.want)
		}
	}
}
