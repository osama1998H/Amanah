package paypal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	sandboxURL     = "https://api-m.sandbox.paypal.com"
	productionURL  = "https://api-m.paypal.com"
	defaultTimeout = 30 * time.Second
)

var (
	ErrAuthFailed      = errors.New("authentication failed")
	ErrPaymentFailed   = errors.New("payment failed")
	ErrRefundFailed    = errors.New("refund failed")
	ErrCaptureFailures = errors.New("capture failed")
	ErrNetworkError    = errors.New("network error")
	ErrInvalidAmount   = errors.New("invalid amount")
)

// Client handles PayPal API interactions
type Client struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	HTTPClient   *http.Client
	Sandbox      bool

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// OrderRequest represents a PayPal order creation request
type OrderRequest struct {
	Intent        string         `json:"intent"` // CAPTURE or AUTHORIZE
	PurchaseUnits []PurchaseUnit `json:"purchase_units"`
	Payer         *Payer         `json:"payer,omitempty"`
}

// PurchaseUnit represents a purchase unit in an order
type PurchaseUnit struct {
	ReferenceID string  `json:"reference_id,omitempty"`
	Amount      Amount  `json:"amount"`
	Description string  `json:"description,omitempty"`
}

// Amount represents a monetary amount
type Amount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"` // Decimal string, e.g., "10.00"
}

// Payer represents the payer information
type Payer struct {
	EmailAddress string `json:"email_address,omitempty"`
}

// OrderResponse represents a PayPal order response
type OrderResponse struct {
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	Intent        string        `json:"intent"`
	PurchaseUnits []PurchaseUnit `json:"purchase_units"`
	CreateTime    string        `json:"create_time"`
	UpdateTime    string        `json:"update_time"`
	Links         []Link        `json:"links"`
}

// Link represents a HATEOAS link
type Link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

// CaptureResponse represents a capture response
type CaptureResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Amount        Amount `json:"amount"`
	FinalCapture  bool   `json:"final_capture"`
}

// RefundRequest represents a refund request
type RefundRequest struct {
	Amount     *Amount `json:"amount,omitempty"`
	InvoiceID  string  `json:"invoice_id,omitempty"`
	NoteToPayer string `json:"note_to_payer,omitempty"`
}

// RefundResponse represents a refund response
type RefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount Amount `json:"amount"`
}

// TokenResponse represents OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewClient creates a new PayPal client
func NewClient(clientID, clientSecret string, sandbox bool) *Client {
	baseURL := productionURL
	if sandbox {
		baseURL = sandboxURL
	}

	return &Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		Sandbox:      sandbox,
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// CreateOrder creates a new PayPal order
func (c *Client) CreateOrder(req *OrderRequest) (*OrderResponse, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v2/checkout/orders", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, token)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrPaymentFailed, resp.StatusCode, string(respBody))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, err
	}

	return &orderResp, nil
}

// CaptureOrder captures an approved order
func (c *Client) CaptureOrder(orderID string) (*OrderResponse, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v2/checkout/orders/"+orderID+"/capture", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, token)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrCaptureFailures, resp.StatusCode, string(respBody))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, err
	}

	return &orderResp, nil
}

// GetOrder retrieves an order by ID
func (c *Client) GetOrder(orderID string) (*OrderResponse, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("GET", c.BaseURL+"/v2/checkout/orders/"+orderID, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, token)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get order: status %d", resp.StatusCode)
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, err
	}

	return &orderResp, nil
}

// RefundCapture refunds a captured payment
func (c *Client) RefundCapture(captureID string, req *RefundRequest) (*RefundResponse, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	var body []byte
	if req != nil {
		body, _ = json.Marshal(req)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v2/payments/captures/"+captureID+"/refund", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq, token)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrRefundFailed, resp.StatusCode, string(respBody))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(respBody, &refundResp); err != nil {
		return nil, err
	}

	return &refundResp, nil
}

// getAccessToken retrieves or refreshes the OAuth access token
func (c *Client) getAccessToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	// Request new token
	data := "grant_type=client_credentials"
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/v1/oauth2/token", bytes.NewBufferString(data))
	if err != nil {
		return "", err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))
	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrAuthFailed
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return c.accessToken, nil
}

// setHeaders sets common headers for PayPal requests
func (c *Client) setHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PayPal-Request-Id", fmt.Sprintf("%d", time.Now().UnixNano()))
}

// Healthcheck verifies the credentials are valid
func (c *Client) Healthcheck() error {
	_, err := c.getAccessToken()
	return err
}
