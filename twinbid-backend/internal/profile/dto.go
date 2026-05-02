package profile

import "encoding/json"

type PatchProfileRequest struct {
	Login                        *string  `json:"login"`
	Mail                         *string  `json:"mail"`
	Name                         *string  `json:"name"`
	Telegram                     *string  `json:"-"`
	TelegramSet                  bool     `json:"-"`
	ManagerTelegram              *string  `json:"manager_telegram"`
	Balance                      *float64 `json:"balance"`
	Timezone                     *string  `json:"timezone"`
	EmailNotifications           *bool    `json:"email_notifications"`
	CampaignStatusNotifications  *bool    `json:"campaign_status_notifications"`
	LowBalanceNotifications      *bool    `json:"low_balance_notifications"`
	CampaignBalanseNotifications *bool    `json:"campaign_balanse_notifications"`
	BalanceTreshold              *float64 `json:"balance_treshold"`
}

type PatchProfileAdminRequest struct {
	UserID string `json:"user_id"`
	PatchProfileRequest
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
	if v, ok := raw["telegram"]; ok {
		aux.TelegramSet = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			aux.Telegram = &s
		}
	}
	*p = PatchProfileRequest(aux)
	return nil
}
