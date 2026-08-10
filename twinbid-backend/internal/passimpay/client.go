package passimpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"twinbid-backend/internal/payments"
)

const maxResponseBody = 1 << 20

type Client struct {
	baseURL     string
	createPath  string
	statusPath  string
	platformID  int64
	apiKey      string
	invoiceType int
	currencyIDs []int
	httpClient  *http.Client
}

var _ payments.InvoiceProvider = (*Client)(nil)

type CreateInvoiceResult = payments.CreateInvoiceResult

type InvoiceStatus = payments.InvoiceStatus

type Config struct {
	BaseURL           string
	PlatformID        int64
	APIKey            string
	CreateInvoicePath string
	CheckInvoicePath  string
	InvoiceType       int
	CurrencyIDs       string
	Timeout           time.Duration
}

func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		createPath:  normalizePath(cfg.CreateInvoicePath, "/v2/createorder"),
		statusPath:  normalizePath(cfg.CheckInvoicePath, "/v2/orderstatus"),
		platformID:  cfg.PlatformID,
		apiKey:      strings.TrimSpace(cfg.APIKey),
		invoiceType: cfg.InvoiceType,
		currencyIDs: parseIDs(cfg.CurrencyIDs),
		httpClient:  &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return payments.ProviderPassimPay }

func (c *Client) PaymentChannel() string { return payments.ChannelPassimPayInvoice }

func (c *Client) Enabled() bool {
	return c != nil && c.platformID > 0 && c.apiKey != "" && c.baseURL != ""
}

func (c *Client) CreateInvoice(ctx context.Context, req payments.CreateInvoiceRequest) (CreateInvoiceResult, error) {
	if !c.Enabled() {
		return CreateInvoiceResult{}, errors.New("PassimPay is not configured")
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}
	payload := map[string]any{
		"platformId": c.platformID,
		"orderId":    req.OrderID,
		"amount":     fmt.Sprintf("%.2f", invoiceAmount(req.Amount)),
		"symbol":     currency,
	}
	if c.invoiceType >= 0 {
		payload["type"] = c.invoiceType
	}
	if len(c.currencyIDs) > 0 {
		payload["currencies"] = c.currencyIDs
	}

	body, err := c.doSignedJSON(ctx, c.createPath, payload)
	if err != nil {
		return CreateInvoiceResult{}, err
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return CreateInvoiceResult{}, fmt.Errorf("decode PassimPay create invoice response: %w", err)
	}
	if err := requireSuccess(response); err != nil {
		return CreateInvoiceResult{}, err
	}

	paymentURL := firstString(response, "url", "paymentUrl", "payment_url", "link")
	if paymentURL == "" {
		return CreateInvoiceResult{}, errors.New("PassimPay create invoice response does not contain payment URL")
	}

	return CreateInvoiceResult{
		PaymentURL:            paymentURL,
		ProviderPaymentID:     firstString(response, "paymentId", "payment_id"),
		ProviderTransactionID: firstString(response, "transactionId", "transaction_id", "invoiceId", "invoice_id"),
		ProviderStatus:        normalizeStatus(firstString(response, "status", "invoiceStatus", "invoice_status")),
		Raw:                   append(json.RawMessage(nil), body...),
	}, nil
}

func (c *Client) CheckInvoice(ctx context.Context, orderID string) (InvoiceStatus, error) {
	if !c.Enabled() {
		return InvoiceStatus{}, errors.New("PassimPay is not configured")
	}

	body, err := c.doSignedJSON(ctx, c.statusPath, map[string]any{
		"platformId": c.platformID,
		"orderId":    orderID,
	})
	if err != nil {
		return InvoiceStatus{}, err
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return InvoiceStatus{}, fmt.Errorf("decode PassimPay invoice status response: %w", err)
	}
	if err := requireSuccess(response); err != nil {
		return InvoiceStatus{}, err
	}

	return ParseInvoiceStatus(response, body), nil
}

func (c *Client) VerifyPayload(raw []byte, receivedSignature string) error {
	if !c.Enabled() {
		return errors.New("PassimPay is not configured")
	}
	if strings.TrimSpace(receivedSignature) == "" {
		return errors.New("missing x-signature")
	}
	canonical, payload, err := canonicalPayload(raw)
	if err != nil {
		return err
	}
	if value, ok := payload["platformId"]; ok {
		platformID, err := int64Value(value)
		if err != nil || platformID != c.platformID {
			return fmt.Errorf("unexpected platformId")
		}
	}
	expected := signature(c.platformID, canonical, c.apiKey)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expected)), []byte(strings.ToLower(strings.TrimSpace(receivedSignature)))) != 1 {
		return errors.New("invalid x-signature")
	}
	return nil
}

func (c *Client) ParseWebhook(raw []byte) (InvoiceStatus, string, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InvoiceStatus{}, "", fmt.Errorf("decode PassimPay webhook: %w", err)
	}
	platformID, err := int64Value(payload["platformId"])
	if err != nil || platformID != c.platformID {
		return InvoiceStatus{}, "", errors.New("PassimPay webhook contains invalid platformId")
	}
	orderID := firstString(payload, "orderId", "order_id")
	if orderID == "" {
		return InvoiceStatus{}, "", errors.New("PassimPay webhook does not contain orderId")
	}
	status := ParseInvoiceStatus(payload, raw)
	// A generic deposit callback is not authoritative proof that a fixed-amount
	// invoice is fully paid: it can also represent a partial payment. The caller
	// must confirm the invoice through the status endpoint before crediting.
	if status.Status == "" {
		status.Status = "waiting"
	}
	return status, orderID, nil
}

func (c *Client) ParseAndVerifyWebhook(raw []byte, headers http.Header) (payments.WebhookEvent, error) {
	signature := strings.TrimSpace(headers.Get("x-signature"))
	if err := c.VerifyPayload(raw, signature); err != nil {
		return payments.WebhookEvent{}, err
	}
	status, orderID, err := c.ParseWebhook(raw)
	if err != nil {
		return payments.WebhookEvent{}, err
	}
	return payments.WebhookEvent{OrderID: orderID, Signature: signature, Status: status}, nil
}

func ParseInvoiceStatus(payload map[string]any, raw []byte) InvoiceStatus {
	status := normalizeStatus(firstString(payload, "status", "invoiceStatus", "invoice_status"))
	return InvoiceStatus{
		PaymentURL:            firstString(payload, "url", "paymentUrl", "payment_url", "link"),
		Status:                status,
		ProviderPaymentID:     firstString(payload, "paymentId", "payment_id"),
		ProviderTransactionID: firstString(payload, "transactionId", "transaction_id", "invoiceId", "invoice_id"),
		TransactionHash:       firstString(payload, "txhash", "transactionHash", "transaction_hash"),
		AmountPaid:            firstFloat(payload, "amountPaid", "amount_paid", "amount"),
		AmountCredited:        firstFloat(payload, "amountCreditedUser", "amountCreditedMerchant", "amountReceive", "amount_credited", "amountCredited"),
		FeeService:            firstFloat(payload, "feeService", "fee_service"),
		FeeNetwork:            firstFloat(payload, "feeNetwork", "fee_network"),
		Raw:                   append(json.RawMessage(nil), raw...),
	}
}

func (c *Client) doSignedJSON(ctx context.Context, path string, payload map[string]any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-signature", signature(c.platformID, body, c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PassimPay request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("read PassimPay response: %w", err)
	}
	if len(responseBody) > maxResponseBody {
		return nil, errors.New("PassimPay response is too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("PassimPay returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

func signature(platformID int64, body []byte, secret string) string {
	contract := strconv.FormatInt(platformID, 10) + ";" + string(body) + ";" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(contract))
	return hex.EncodeToString(mac.Sum(nil))
}

func canonicalPayload(raw []byte) ([]byte, map[string]any, error) {
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("decode signed payload: %w", err)
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	return canonical, payload, nil
}

func int64Value(v any) (int64, error) {
	switch value := v.(type) {
	case json.Number:
		return value.Int64()
	case float64:
		return int64(value), nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, errors.New("unsupported number")
	}
}

func requireSuccess(payload map[string]any) error {
	value, ok := payload["result"]
	if !ok {
		return nil
	}
	success := false
	switch v := value.(type) {
	case float64:
		success = v == 1
	case json.Number:
		success = v.String() == "1"
	case string:
		success = v == "1"
	case bool:
		success = v
	}
	if success {
		return nil
	}
	message := firstString(payload, "message", "error", "errorMessage")
	if message == "" {
		message = "PassimPay returned result=0"
	}
	return errors.New(message)
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "paid", "success", "successful", "approved", "1":
		return "paid"
	case "wait", "waiting", "pending", "0":
		return "waiting"
	case "error", "failed", "rejected", "expired", "2":
		return "error"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case json.Number:
			return v.String()
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	return ""
}

func firstFloat(payload map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		var parsed float64
		var err error
		switch v := value.(type) {
		case float64:
			parsed = v
		case json.Number:
			parsed, err = v.Float64()
		case string:
			parsed, err = strconv.ParseFloat(strings.TrimSpace(v), 64)
		default:
			continue
		}
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func invoiceAmount(depositAmount float64) float64 {
	return math.Round(depositAmount*1.01*100) / 100
}

func normalizePath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}

func parseIDs(raw string) []int {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && value > 0 {
			out = append(out, value)
		}
	}
	return out
}
