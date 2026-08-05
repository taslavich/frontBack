package profile

import (
	"encoding/json"
	"fmt"
)

type PatchProfileRequest struct {
	Login                        *string  `json:"login"`
	Mail                         *string  `json:"mail"`
	Name                         *string  `json:"name"`
	Telegram                     *string  `json:"-"`
	TelegramSet                  bool     `json:"-"`
	ManagerTelegram              *string  `json:"manager_telegram"`
	Timezone                     *string  `json:"timezone"`
	EmailNotifications           *bool    `json:"email_notifications"`
	CampaignStatusNotifications  *bool    `json:"campaign_status_notifications"`
	LowBalanceNotifications      *bool    `json:"low_balance_notifications"`
	CampaignBalanseNotifications *bool    `json:"campaign_balanse_notifications"`
	BalanceTreshold              *float64 `json:"balance_treshold"`
}

func (p *PatchProfileRequest) UnmarshalJSON(data []byte) error {
	type alias PatchProfileRequest
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, ok := raw["balance"]; ok {
		return fmt.Errorf("balance cannot be changed through profile API")
	}
	if v, ok := raw["telegram"]; ok {
		aux.TelegramSet = true
		if string(v) != "null" {
			var value string
			if err := json.Unmarshal(v, &value); err != nil {
				return err
			}
			aux.Telegram = &value
		}
	}
	*p = PatchProfileRequest(aux)
	return nil
}
