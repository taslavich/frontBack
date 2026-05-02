package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

type BotClient struct {
	BaseURL        string
	InternalSecret string
	Client         *http.Client
}

func NewBotClient(baseURL, internalSecret string) *BotClient {
	return &BotClient{
		BaseURL:        strings.TrimRight(baseURL, "/"),
		InternalSecret: internalSecret,
		Client:         &http.Client{Timeout: 10 * time.Second},
	}
}

type FilePayload struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

type CreativePayload struct {
	CreativeName string       `json:"creative_name"`
	URL          string       `json:"url"`
	Macros       string       `json:"macros,omitempty"`
	ImageFile    *FilePayload `json:"-"`
	ImageURL     string       `json:"image_url,omitempty"`
	Title        string       `json:"title,omitempty"`
	Description  string       `json:"description,omitempty"`
}

type CampaignModerationRequest struct {
	CampaignID   string            `json:"campaign_id"`
	FormatType   string            `json:"format_type"`  // popunder / banner / native / push
	TrafficType  string            `json:"traffic_type"` // mainstream / adult / mixed и т.п.
	CampaignName string            `json:"campaign_name"`
	BannerSize   string            `json:"banner_size,omitempty"` // нужно для banner, например 300x250
	BrandName    string            `json:"brand_name,omitempty"`  // нужно для native/push
	UserID       string            `json:"user_id"`
	UserEmail    string            `json:"user_email"`
	UserTelegram string            `json:"user_telegram,omitempty"`
	Creatives    []CreativePayload `json:"creatives"`
}

type PaymentModerationRequest struct {
	ID                   string  `json:"id"`             // user_transactions.id, именно он идет в /api/transactions/{id}/approve_admin
	TransactionID        string  `json:"transaction_id"` // внешний/публичный transaction_id, только для отображения
	UserID               string  `json:"user_id"`
	UserEmail            string  `json:"user_email"`
	UserTelegram         string  `json:"user_telegram,omitempty"`
	PaymentMethod        string  `json:"payment_method,omitempty"`
	DepositAmount        float64 `json:"deposit_amount"`         // сумма пополнения
	BonusAmount          float64 `json:"bonus_amount,omitempty"` // бонус, если есть
	TotalBalanceIncrease float64 `json:"total_balance_increase"` // конечная сумма начисления
	Currency             string  `json:"currency"`
	PromocodeID          string  `json:"promocode_id,omitempty"`
	TransactionHash      string  `json:"transaction_hash"`
}

func (b *BotClient) SendCampaignModeration(ctx context.Context, req CampaignModerationRequest) error {
	if campaignHasFiles(req) {
		return b.postCampaignMultipart(ctx, req)
	}
	return b.postJSON(ctx, "/internal/campaigns/moderation", req)
}

func (b *BotClient) SendPaymentModeration(ctx context.Context, req PaymentModerationRequest) error {
	return b.postJSON(ctx, "/internal/payments/moderation", req)
}

func campaignHasFiles(req CampaignModerationRequest) bool {
	for _, cr := range req.Creatives {
		if cr.ImageFile != nil {
			return true
		}
	}
	return false
}

func (b *BotClient) postCampaignMultipart(ctx context.Context, req CampaignModerationRequest) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := writer.WriteField("payload", string(payload)); err != nil {
		return err
	}

	for i, cr := range req.Creatives {
		if cr.ImageFile == nil {
			continue
		}
		if cr.ImageFile.Reader == nil {
			return fmt.Errorf("creatives[%d].ImageFile.Reader is nil", i)
		}

		filename := strings.TrimSpace(cr.ImageFile.Filename)
		if filename == "" {
			filename = fmt.Sprintf("creative_image_%d", i)
		}

		partHeader := make(textproto.MIMEHeader)
		partHeader.Set(
			"Content-Disposition",
			fmt.Sprintf(`form-data; name="creative_image_%d"; filename="%s"`, i, escapeQuotes(filename)),
		)

		if strings.TrimSpace(cr.ImageFile.ContentType) != "" {
			partHeader.Set("Content-Type", cr.ImageFile.ContentType)
		} else {
			partHeader.Set("Content-Type", "application/octet-stream")
		}

		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return err
		}

		if _, err := io.Copy(part, cr.ImageFile.Reader); err != nil {
			return err
		}
	}

	if err := writer.Close(); err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		b.BaseURL+"/internal/campaigns/moderation",
		&body,
	)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("X-Bot-Secret", b.InternalSecret)

	return b.do(httpReq)
}

func (b *BotClient) postJSON(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		b.BaseURL+path,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Bot-Secret", b.InternalSecret)

	return b.do(httpReq)
}

func (b *BotClient) do(req *http.Request) error {
	resp, err := b.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram moderation bot failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return nil
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
