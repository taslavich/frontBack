package topups

type CreateTopupRequest struct {
	PaymentMethod   string  `json:"payment_method"`
	DepositAmount   float64 `json:"deposit_amount"`
	Currency        string  `json:"currency"`
	PromocodeID     *string `json:"promocode_id"`
	BonusAmount     float64 `json:"bonus_amount"`
	TransactionHash *string `json:"transaction_hash"`
	Status          string  `json:"status"`
}
