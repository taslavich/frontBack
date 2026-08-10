package cryptomus

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"twinbid-backend/internal/payments"
)

const maxResponseBody = 1 << 20

type Config struct {
	BaseURL           string
	MerchantUUID      string
	PaymentAPIKey     string
	CreateInvoicePath string
	CheckInvoicePath  string
	WebhookURL        string
	SubtractPercent   int
	Timeout           time.Duration
}

type Client struct {
	baseURL         string
	createPath      string
	statusPath      string
	merchantUUID    string
	paymentAPIKey   string
	webhookURL      string
	subtractPercent int
	httpClient      *http.Client
}

var _ payments.InvoiceProvider = (*Client)(nil)

type paymentResult struct {
	UUID           string `json:"uuid"`
	OrderID        string `json:"order_id"`
	Amount         string `json:"amount"`
	PaymentAmount  string `json:"payment_amount"`
	MerchantAmount string `json:"merchant_amount"`
	Commission     string `json:"commission"`
	PaymentStatus  string `json:"payment_status"`
	Status         string `json:"status"`
	URL            string `json:"url"`
	TxID           string `json:"txid"`
	TransferID     string `json:"transfer_id"`
	IsFinal        *bool  `json:"is_final"`
}

type apiEnvelope struct {
	State   int             `json:"state"`
	Result  json.RawMessage `json:"result"`
	Errors  json.RawMessage `json:"errors"`
	Message string          `json:"message"`
}

func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	subtract := cfg.SubtractPercent
	if subtract < 0 {
		subtract = 0
	}
	if subtract > 100 {
		subtract = 100
	}
	return &Client{
		baseURL:         strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		createPath:      normalizePath(cfg.CreateInvoicePath, "/v1/payment"),
		statusPath:      normalizePath(cfg.CheckInvoicePath, "/v1/payment/info"),
		merchantUUID:    strings.TrimSpace(cfg.MerchantUUID),
		paymentAPIKey:   strings.TrimSpace(cfg.PaymentAPIKey),
		webhookURL:      strings.TrimSpace(cfg.WebhookURL),
		subtractPercent: subtract,
		httpClient:      &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return payments.ProviderCryptomus }

func (c *Client) PaymentChannel() string { return payments.ChannelCryptomusInvoice }

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != "" && c.merchantUUID != "" && c.paymentAPIKey != "" && c.webhookURL != ""
}

func (c *Client) CreateInvoice(ctx context.Context, req payments.CreateInvoiceRequest) (payments.CreateInvoiceResult, error) {
	if !c.Enabled() {
		return payments.CreateInvoiceResult{}, errors.New("Cryptomus is not configured")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}
	lifetimeSeconds := int(req.Lifetime / time.Second)
	if lifetimeSeconds <= 0 {
		lifetimeSeconds = 3600
	}
	payload := struct {
		Amount      string `json:"amount"`
		Currency    string `json:"currency"`
		OrderID     string `json:"order_id"`
		URLCallback string `json:"url_callback"`
		Subtract    int    `json:"subtract"`
		Lifetime    int    `json:"lifetime"`
	}{
		Amount:      fmt.Sprintf("%.2f", req.Amount),
		Currency:    currency,
		OrderID:     req.OrderID,
		URLCallback: c.webhookURL,
		Subtract:    c.subtractPercent,
		Lifetime:    lifetimeSeconds,
	}

	body, resultRaw, err := c.doSignedJSON(ctx, c.createPath, payload)
	if err != nil {
		return payments.CreateInvoiceResult{}, err
	}
	var result paymentResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return payments.CreateInvoiceResult{}, fmt.Errorf("decode Cryptomus create invoice result: %w", err)
	}
	if strings.TrimSpace(result.URL) == "" {
		return payments.CreateInvoiceResult{}, errors.New("Cryptomus create invoice response does not contain payment URL")
	}
	return payments.CreateInvoiceResult{
		PaymentURL:            strings.TrimSpace(result.URL),
		ProviderPaymentID:     strings.TrimSpace(result.UUID),
		ProviderTransactionID: strings.TrimSpace(result.TransferID),
		ProviderStatus:        normalizePaymentStatus(firstNonEmpty(result.PaymentStatus, result.Status), result.IsFinal),
		Raw:                   append(json.RawMessage(nil), body...),
	}, nil
}

func (c *Client) CheckInvoice(ctx context.Context, orderID string) (payments.InvoiceStatus, error) {
	if !c.Enabled() {
		return payments.InvoiceStatus{}, errors.New("Cryptomus is not configured")
	}
	payload := struct {
		OrderID string `json:"order_id"`
	}{OrderID: orderID}
	body, resultRaw, err := c.doSignedJSON(ctx, c.statusPath, payload)
	if err != nil {
		return payments.InvoiceStatus{}, err
	}
	var result paymentResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return payments.InvoiceStatus{}, fmt.Errorf("decode Cryptomus payment info result: %w", err)
	}
	return resultToInvoiceStatus(result, body), nil
}

func (c *Client) ParseAndVerifyWebhook(raw []byte, _ http.Header) (payments.WebhookEvent, error) {
	if !c.Enabled() {
		return payments.WebhookEvent{}, errors.New("Cryptomus is not configured")
	}
	var payload struct {
		Type           string `json:"type"`
		UUID           string `json:"uuid"`
		OrderID        string `json:"order_id"`
		PaymentAmount  string `json:"payment_amount"`
		MerchantAmount string `json:"merchant_amount"`
		Commission     string `json:"commission"`
		Status         string `json:"status"`
		TxID           string `json:"txid"`
		TransferID     string `json:"transfer_id"`
		IsFinal        *bool  `json:"is_final"`
		Sign           string `json:"sign"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payments.WebhookEvent{}, fmt.Errorf("decode Cryptomus webhook: %w", err)
	}
	if strings.TrimSpace(payload.Sign) == "" {
		return payments.WebhookEvent{}, errors.New("Cryptomus webhook does not contain sign")
	}
	if strings.TrimSpace(payload.OrderID) == "" {
		return payments.WebhookEvent{}, errors.New("Cryptomus webhook does not contain order_id")
	}
	if payload.Type != "" && !strings.EqualFold(strings.TrimSpace(payload.Type), "payment") {
		return payments.WebhookEvent{}, errors.New("Cryptomus webhook type is not payment")
	}
	if err := c.verifyWebhookSignature(raw, payload.Sign); err != nil {
		return payments.WebhookEvent{}, err
	}

	status := payments.InvoiceStatus{
		Status:                normalizePaymentStatus(payload.Status, payload.IsFinal),
		ProviderPaymentID:     strings.TrimSpace(payload.UUID),
		ProviderTransactionID: strings.TrimSpace(payload.TransferID),
		TransactionHash:       strings.TrimSpace(payload.TxID),
		AmountPaid:            parseFloatPtr(payload.PaymentAmount),
		AmountCredited:        parseFloatPtr(payload.MerchantAmount),
		FeeService:            parseFloatPtr(payload.Commission),
		Raw:                   append(json.RawMessage(nil), raw...),
	}
	return payments.WebhookEvent{
		OrderID:   strings.TrimSpace(payload.OrderID),
		Signature: strings.TrimSpace(payload.Sign),
		Status:    status,
	}, nil
}

func resultToInvoiceStatus(result paymentResult, raw []byte) payments.InvoiceStatus {
	return payments.InvoiceStatus{
		PaymentURL:            strings.TrimSpace(result.URL),
		Status:                normalizePaymentStatus(firstNonEmpty(result.PaymentStatus, result.Status), result.IsFinal),
		ProviderPaymentID:     strings.TrimSpace(result.UUID),
		ProviderTransactionID: strings.TrimSpace(result.TransferID),
		TransactionHash:       strings.TrimSpace(result.TxID),
		AmountPaid:            parseFloatPtr(result.PaymentAmount),
		AmountCredited:        parseFloatPtr(result.MerchantAmount),
		FeeService:            parseFloatPtr(result.Commission),
		Raw:                   append(json.RawMessage(nil), raw...),
	}
}

func normalizePaymentStatus(status string, isFinal *bool) string {
	value := strings.ToLower(strings.TrimSpace(status))
	switch value {
	case "paid", "paid_over":
		// Money is credited only after Cryptomus explicitly marks the invoice final.
		// A missing is_final field is treated conservatively as non-final.
		if isFinal == nil || !*isFinal {
			return "waiting"
		}
		return "paid"
	case "wrong_amount", "fail", "cancel", "system_fail", "refund_fail", "refund_paid":
		return "error"
	case "process", "confirm_check", "wrong_amount_waiting", "check", "refund_process", "locked", "":
		return "waiting"
	default:
		// Unknown provider statuses are deliberately non-crediting. Reconciliation
		// keeps polling until a recognized terminal/success state is returned.
		return "waiting"
	}
}

func (c *Client) doSignedJSON(ctx context.Context, path string, payload any) ([]byte, json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("merchant", c.merchantUUID)
	req.Header.Set("sign", requestSignature(body, c.paymentAPIKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Cryptomus request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read Cryptomus response: %w", err)
	}
	if len(responseBody) > maxResponseBody {
		return nil, nil, errors.New("Cryptomus response is too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("Cryptomus returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode Cryptomus response: %w", err)
	}
	if envelope.State != 0 || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = strings.TrimSpace(string(envelope.Errors))
		}
		if message == "" || message == "null" {
			message = "Cryptomus returned unsuccessful state"
		}
		return nil, nil, errors.New(message)
	}
	return responseBody, append(json.RawMessage(nil), envelope.Result...), nil
}

func requestSignature(body []byte, apiKey string) string {
	encoded := base64.StdEncoding.EncodeToString(body)
	sum := md5.Sum([]byte(encoded + apiKey))
	return hex.EncodeToString(sum[:])
}

func (c *Client) verifyWebhookSignature(raw []byte, received string) error {
	canonical, err := compactObjectWithoutField(raw, "sign")
	if err != nil {
		return err
	}
	candidates := [][]byte{canonical}
	withEscapedSlashes := escapeUnescapedSlashes(canonical)
	if !bytes.Equal(withEscapedSlashes, canonical) {
		candidates = append(candidates, withEscapedSlashes)
	}
	for _, candidate := range candidates {
		expected := requestSignature(candidate, c.paymentAPIKey)
		if constantTimeHexEqual(expected, received) {
			return nil
		}
	}
	return errors.New("invalid Cryptomus signature")
}

func compactObjectWithoutField(raw []byte, field string) ([]byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' {
		return nil, errors.New("Cryptomus webhook must be a JSON object")
	}
	pos := 1
	parts := make([][]byte, 0, 16)
	found := false
	for {
		pos = skipSpace(raw, pos)
		if pos >= len(raw) {
			return nil, errors.New("unterminated Cryptomus webhook object")
		}
		if raw[pos] == '}' {
			pos++
			break
		}
		itemStart := pos
		keyEnd, err := scanJSONString(raw, pos)
		if err != nil {
			return nil, err
		}
		var key string
		if err := json.Unmarshal(raw[pos:keyEnd], &key); err != nil {
			return nil, fmt.Errorf("decode Cryptomus webhook key: %w", err)
		}
		pos = skipSpace(raw, keyEnd)
		if pos >= len(raw) || raw[pos] != ':' {
			return nil, errors.New("invalid Cryptomus webhook object")
		}
		pos = skipSpace(raw, pos+1)
		valueEnd, err := scanJSONValue(raw, pos)
		if err != nil {
			return nil, err
		}
		itemEnd := valueEnd
		if key == field {
			if found {
				return nil, errors.New("duplicate Cryptomus webhook sign field")
			}
			found = true
		} else {
			parts = append(parts, bytes.TrimSpace(raw[itemStart:itemEnd]))
		}
		pos = skipSpace(raw, valueEnd)
		if pos >= len(raw) {
			return nil, errors.New("unterminated Cryptomus webhook object")
		}
		if raw[pos] == ',' {
			pos++
			continue
		}
		if raw[pos] == '}' {
			pos++
			break
		}
		return nil, errors.New("invalid Cryptomus webhook object separator")
	}
	if !found {
		return nil, errors.New("Cryptomus webhook does not contain sign")
	}
	if skipSpace(raw, pos) != len(raw) {
		return nil, errors.New("unexpected data after Cryptomus webhook object")
	}
	joined := append([]byte{'{'}, bytes.Join(parts, []byte{','})...)
	joined = append(joined, '}')
	var compact bytes.Buffer
	if err := json.Compact(&compact, joined); err != nil {
		return nil, fmt.Errorf("compact Cryptomus webhook: %w", err)
	}
	return compact.Bytes(), nil
}

func scanJSONValue(raw []byte, pos int) (int, error) {
	pos = skipSpace(raw, pos)
	if pos >= len(raw) {
		return 0, errors.New("missing Cryptomus webhook value")
	}
	switch raw[pos] {
	case '"':
		return scanJSONString(raw, pos)
	case '{', '[':
		open := raw[pos]
		close := byte('}')
		if open == '[' {
			close = ']'
		}
		depth := 1
		for i := pos + 1; i < len(raw); i++ {
			switch raw[i] {
			case '"':
				end, err := scanJSONString(raw, i)
				if err != nil {
					return 0, err
				}
				i = end - 1
			case open:
				depth++
			case close:
				depth--
				if depth == 0 {
					return i + 1, nil
				}
			case '{', '[':
				// Nested containers of a different kind need a generic stack. Fall
				// back to the stack scanner below.
				return scanJSONContainer(raw, pos)
			}
		}
		return 0, errors.New("unterminated Cryptomus webhook container")
	default:
		i := pos
		for i < len(raw) && raw[i] != ',' && raw[i] != '}' && raw[i] != ']' {
			i++
		}
		if i == pos {
			return 0, errors.New("invalid Cryptomus webhook value")
		}
		return i, nil
	}
}

func scanJSONContainer(raw []byte, pos int) (int, error) {
	stack := []byte{raw[pos]}
	for i := pos + 1; i < len(raw); i++ {
		switch raw[i] {
		case '"':
			end, err := scanJSONString(raw, i)
			if err != nil {
				return 0, err
			}
			i = end - 1
		case '{', '[':
			stack = append(stack, raw[i])
		case '}', ']':
			if len(stack) == 0 {
				return 0, errors.New("invalid Cryptomus webhook container")
			}
			open := stack[len(stack)-1]
			if (open == '{' && raw[i] != '}') || (open == '[' && raw[i] != ']') {
				return 0, errors.New("mismatched Cryptomus webhook container")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, errors.New("unterminated Cryptomus webhook container")
}

func scanJSONString(raw []byte, pos int) (int, error) {
	if pos >= len(raw) || raw[pos] != '"' {
		return 0, errors.New("invalid Cryptomus webhook string")
	}
	escaped := false
	for i := pos + 1; i < len(raw); i++ {
		if escaped {
			escaped = false
			continue
		}
		if raw[i] == '\\' {
			escaped = true
			continue
		}
		if raw[i] == '"' {
			return i + 1, nil
		}
	}
	return 0, errors.New("unterminated Cryptomus webhook string")
}

func escapeUnescapedSlashes(raw []byte) []byte {
	out := make([]byte, 0, len(raw)+8)
	inString := false
	escaped := false
	for _, b := range raw {
		if !inString {
			out = append(out, b)
			if b == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			out = append(out, b)
			escaped = false
			continue
		}
		if b == '\\' {
			out = append(out, b)
			escaped = true
			continue
		}
		if b == '"' {
			out = append(out, b)
			inString = false
			continue
		}
		if b == '/' {
			out = append(out, '\\', '/')
			continue
		}
		out = append(out, b)
	}
	return out
}

func constantTimeHexEqual(expected, received string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	received = strings.ToLower(strings.TrimSpace(received))
	return len(expected) == len(received) && subtle.ConstantTimeCompare([]byte(expected), []byte(received)) == 1
}

func skipSpace(raw []byte, pos int) int {
	for pos < len(raw) {
		switch raw[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func parseFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
