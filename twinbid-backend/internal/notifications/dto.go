package notifications

import "encoding/json"

type CreateNotificationRequest struct {
	TransactionID *string  `json:"transaction_id"`
	CampaignID    *string  `json:"campaign_id"`
	DepositAmount *float64 `json:"deposit_amount"`
	Text          string   `json:"text"`
	Type          string   `json:"type"`
}

type PatchNotificationRequest struct {
	TransactionID    *string  `json:"-"`
	TransactionIDSet bool     `json:"-"`
	CampaignID       *string  `json:"-"`
	CampaignIDSet    bool     `json:"-"`
	DepositAmount    *float64 `json:"-"`
	DepositAmountSet bool     `json:"-"`
	Status           *string  `json:"status"`
	Text             *string  `json:"text"`
	Type             *string  `json:"type"`
}

func (p *PatchNotificationRequest) UnmarshalJSON(data []byte) error {
	type alias PatchNotificationRequest
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["transaction_id"]; ok {
		aux.TransactionIDSet = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			aux.TransactionID = &s
		}
	}
	if v, ok := raw["campaign_id"]; ok {
		aux.CampaignIDSet = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			aux.CampaignID = &s
		}
	}
	if v, ok := raw["deposit_amount"]; ok {
		aux.DepositAmountSet = true
		if string(v) != "null" {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			aux.DepositAmount = &f
		}
	}
	*p = PatchNotificationRequest(aux)
	return nil
}
