package tronscan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBody = 2 << 20

type Config struct {
	BaseURL          string
	APIKey           string
	WalletAddress    string
	USDTContract     string
	Timeout          time.Duration
	MinConfirmations int64
}

type Client struct {
	baseURL          string
	apiKey           string
	walletAddress    string
	usdtContract     string
	minConfirmations int64
	httpClient       *http.Client
}

type VerificationState string

const (
	VerificationPending  VerificationState = "pending"
	VerificationVerified VerificationState = "verified"
	VerificationInvalid  VerificationState = "invalid"
)

type Verification struct {
	State         VerificationState
	Reason        string
	Confirmations int64
	AmountMicro   *big.Int
	Raw           json.RawMessage
}

type transactionResponse struct {
	Hash              string         `json:"hash"`
	Confirmed         bool           `json:"confirmed"`
	Revert            bool           `json:"revert"`
	Confirmations     int64          `json:"confirmations"`
	ContractRet       string         `json:"contractRet"`
	TRC20TransferInfo []transferInfo `json:"trc20TransferInfo"`
}

type transferInfo struct {
	ToAddress       string `json:"to_address"`
	ContractAddress string `json:"contract_address"`
	AmountStr       string `json:"amount_str"`
	Decimals        int    `json:"decimals"`
	Type            string `json:"type"`
	Status          int    `json:"status"`
	TokenType       string `json:"tokenType"`
}

func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	minConfirmations := cfg.MinConfirmations
	if minConfirmations <= 0 {
		minConfirmations = 1
	}
	return &Client{
		baseURL:          strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:           strings.TrimSpace(cfg.APIKey),
		walletAddress:    strings.TrimSpace(cfg.WalletAddress),
		usdtContract:     strings.TrimSpace(cfg.USDTContract),
		minConfirmations: minConfirmations,
		httpClient:       &http.Client{Timeout: timeout},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.walletAddress != "" && c.usdtContract != ""
}

func (c *Client) VerifyUSDTTransfer(ctx context.Context, txHash string, expectedAmountMicro *big.Int) (Verification, error) {
	if !c.Enabled() {
		return Verification{}, errors.New("TronScan client is not configured")
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return Verification{}, errors.New("transaction hash is empty")
	}
	if expectedAmountMicro == nil || expectedAmountMicro.Sign() <= 0 {
		return Verification{}, errors.New("expected amount must be positive")
	}

	endpoint, err := url.Parse(c.baseURL + "/api/transaction-info")
	if err != nil {
		return Verification{}, fmt.Errorf("build TronScan URL: %w", err)
	}
	q := endpoint.Query()
	q.Set("hash", txHash)
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Verification{}, fmt.Errorf("build TronScan request: %w", err)
	}
	req.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Verification{}, fmt.Errorf("TronScan transaction lookup: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return Verification{}, fmt.Errorf("read TronScan response: %w", err)
	}
	if len(body) > maxResponseBody {
		return Verification{}, errors.New("TronScan response is too large")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Verification{}, fmt.Errorf("TronScan returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tx transactionResponse
	if err := json.Unmarshal(body, &tx); err != nil {
		return Verification{}, fmt.Errorf("decode TronScan transaction response: %w", err)
	}
	raw := append(json.RawMessage(nil), body...)
	if strings.TrimSpace(tx.Hash) == "" {
		return Verification{State: VerificationPending, Reason: "transaction not found yet", Raw: raw}, nil
	}
	if !strings.EqualFold(strings.TrimSpace(tx.Hash), txHash) {
		return Verification{}, errors.New("TronScan returned a different transaction hash")
	}
	if !tx.Confirmed || tx.Confirmations < c.minConfirmations {
		return Verification{
			State:         VerificationPending,
			Reason:        fmt.Sprintf("transaction is not confirmed enough: confirmations=%d required=%d", tx.Confirmations, c.minConfirmations),
			Confirmations: tx.Confirmations,
			Raw:           raw,
		}, nil
	}
	if tx.Revert || (strings.TrimSpace(tx.ContractRet) != "" && !strings.EqualFold(strings.TrimSpace(tx.ContractRet), "SUCCESS")) {
		return Verification{State: VerificationInvalid, Reason: "transaction failed or was reverted", Confirmations: tx.Confirmations, Raw: raw}, nil
	}

	total := big.NewInt(0)
	found := false
	for _, transfer := range tx.TRC20TransferInfo {
		if strings.TrimSpace(transfer.ContractAddress) != c.usdtContract {
			continue
		}
		if strings.TrimSpace(transfer.ToAddress) != c.walletAddress {
			continue
		}
		if transfer.Status != 0 || !strings.EqualFold(strings.TrimSpace(transfer.Type), "Transfer") {
			continue
		}
		if transfer.TokenType != "" && !strings.EqualFold(strings.TrimSpace(transfer.TokenType), "trc20") {
			continue
		}
		if transfer.Decimals != 6 {
			continue
		}
		amount := new(big.Int)
		if _, ok := amount.SetString(strings.TrimSpace(transfer.AmountStr), 10); !ok || amount.Sign() < 0 {
			return Verification{}, fmt.Errorf("invalid TronScan USDT amount %q", transfer.AmountStr)
		}
		total.Add(total, amount)
		found = true
	}
	if !found {
		return Verification{
			State:         VerificationInvalid,
			Reason:        "no successful USDT TRC20 transfer to the configured wallet",
			Confirmations: tx.Confirmations,
			AmountMicro:   new(big.Int).Set(total),
			Raw:           raw,
		}, nil
	}
	if total.Cmp(expectedAmountMicro) != 0 {
		return Verification{
			State:         VerificationInvalid,
			Reason:        fmt.Sprintf("USDT amount mismatch: blockchain_micro=%s expected_micro=%s", total.String(), expectedAmountMicro.String()),
			Confirmations: tx.Confirmations,
			AmountMicro:   new(big.Int).Set(total),
			Raw:           raw,
		}, nil
	}

	return Verification{
		State:         VerificationVerified,
		Reason:        "confirmed USDT TRC20 transfer matches wallet and amount",
		Confirmations: tx.Confirmations,
		AmountMicro:   new(big.Int).Set(total),
		Raw:           raw,
	}, nil
}
