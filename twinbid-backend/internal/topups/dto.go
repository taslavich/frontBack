package topups

import "encoding/json"

const (
	PaymentChannelStaticWallet     = "static_wallet"
	PaymentChannelPassimPayInvoice = "passimpay_invoice"
)

type CreateTopupRequest struct {
	PaymentChannel  string  `json:"payment_channel,omitempty"`
	PaymentMethod   string  `json:"payment_method"`
	DepositAmount   float64 `json:"deposit_amount"`
	Currency        string  `json:"currency"`
	PromocodeID     *string `json:"promocode_id"`
	BonusAmount     float64 `json:"bonus_amount,omitempty"`
	TransactionHash *string `json:"transaction_hash,omitempty"`
	Status          string  `json:"status,omitempty"`
}

type AdminTopupActionRequest struct {
	UserID string `json:"user_id"`
}

// PatchTopupRequest remains backward-compatible with the current frontend payload,
// but the service only accepts transaction_hash for static-wallet payments.
type PatchTopupRequest struct {
	TransactionID        *string  `json:"transaction_id,omitempty"`
	PaymentMethod        *string  `json:"payment_method,omitempty"`
	BonusAmount          *float64 `json:"bonus_amount,omitempty"`
	PromocodeID          *string  `json:"promocode_id,omitempty"`
	PromocodeIDSet       bool     `json:"-"`
	TransactionHash      *string  `json:"transaction_hash,omitempty"`
	TransactionHashSet   bool     `json:"-"`
	DepositAmount        *float64 `json:"deposit_amount,omitempty"`
	TotalBalanceIncrease *float64 `json:"total_balance_increase,omitempty"`
	Status               *string  `json:"status,omitempty"`
	Currency             *string  `json:"currency,omitempty"`
}

func (p *PatchTopupRequest) UnmarshalJSON(data []byte) error {
	type alias PatchTopupRequest
	aux := struct {
		alias
		PromocodeID     json.RawMessage `json:"promocode_id"`
		TransactionHash json.RawMessage `json:"transaction_hash"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = PatchTopupRequest(aux.alias)
	if len(aux.PromocodeID) > 0 {
		p.PromocodeIDSet = true
		if string(aux.PromocodeID) != "null" {
			var v string
			if err := json.Unmarshal(aux.PromocodeID, &v); err != nil {
				return err
			}
			p.PromocodeID = &v
		}
	}
	if len(aux.TransactionHash) > 0 {
		p.TransactionHashSet = true
		if string(aux.TransactionHash) != "null" {
			var v string
			if err := json.Unmarshal(aux.TransactionHash, &v); err != nil {
				return err
			}
			p.TransactionHash = &v
		}
	}
	return nil
}
