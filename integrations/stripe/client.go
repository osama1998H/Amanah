package stripe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL        = "https://api.stripe.com/v1"
	defaultTimeout = 30 * time.Second
)

var (
	ErrInvalidAPIKey    = errors.New("invalid API key")
	ErrPaymentFailed    = errors.New("payment failed")
	ErrRefundFailed     = errors.New("refund failed")
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrNetworkError     = errors.New("network error")
)

// Client handles Stripe API interactions
type Client struct {
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string
}

// ChargeRequest represents a charge request to Stripe
type ChargeRequest struct {
	Amount      int64             `json:"amount"`      // Amount in cents
	Currency    string            `json:"currency"`
	Source      string            `json:"source"`      // Payment source token
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IdempotencyKey string         `json:"-"`           // Sent as header
}

// ChargeResponse represents a Stripe charge response
type ChargeResponse struct {
	ID            string `json:"id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	Paid          bool   `json:"paid"`
	RefundedAmount int64 `json:"amount_refunded"`
	FailureCode   string `json:"failure_code,omitempty"`
	FailureMessage string `json:"failure_message,omitempty"`
	Created       int64  `json:"created"`
}

// RefundRequest represents a refund request
type RefundRequest struct {
	ChargeID string `json:"charge"`
	Amount   int64  `json:"amount,omitempty"` // Partial refund, 0 for full
	Reason   string `json:"reason,omitempty"`
}

// RefundResponse represents a Stripe refund response
type RefundResponse struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	ChargeID string `json:"charge"`
	Status   string `json:"status"`
	Created  int64  `json:"created"`
}

// NewClient creates a new Stripe client
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Charge creates a payment charge
func (c *Client) Charge(req *ChargeRequest) (*ChargeResponse, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	// Build form data (Stripe uses form encoding)
	data := fmt.Sprintf("amount=%d&currency=%s&source=%s",
		req.Amount, req.Currency, req.Source)
	if req.Description != "" {
		data += "&description=" + req.Description
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/charges", bytes.NewBufferString(data))
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, req.IdempotencyKey)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrPaymentFailed, resp.StatusCode, string(body))
	}

	var chargeResp ChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, err
	}

	return &chargeResp, nil
}

// Refund creates a refund for a charge
func (c *Client) Refund(req *RefundRequest) (*RefundResponse, error) {
	data := fmt.Sprintf("charge=%s", req.ChargeID)
	if req.Amount > 0 {
		data += fmt.Sprintf("&amount=%d", req.Amount)
	}
	if req.Reason != "" {
		data += "&reason=" + req.Reason
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/refunds", bytes.NewBufferString(data))
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrRefundFailed, resp.StatusCode, string(body))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(body, &refundResp); err != nil {
		return nil, err
	}

	return &refundResp, nil
}

// GetCharge retrieves a charge by ID
func (c *Client) GetCharge(chargeID string) (*ChargeResponse, error) {
	httpReq, err := http.NewRequest("GET", c.BaseURL+"/charges/"+chargeID, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, "")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get charge: status %d", resp.StatusCode)
	}

	var chargeResp ChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, err
	}

	return &chargeResp, nil
}

// setHeaders sets common headers for Stripe requests
func (c *Client) setHeaders(req *http.Request, idempotencyKey string) {
	req.SetBasicAuth(c.APIKey, "")
	req.Header.Set("Stripe-Version", "2023-10-16")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
}

// Healthcheck verifies the API key is valid
func (c *Client) Healthcheck() error {
	httpReq, err := http.NewRequest("GET", c.BaseURL+"/balance", nil)
	if err != nil {
		return err
	}
	c.setHeaders(httpReq, "")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return ErrNetworkError
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrInvalidAPIKey
	}

	return nil
}
