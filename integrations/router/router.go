package router

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"amanah/integrations/paypal"
	"amanah/integrations/stripe"
)

var (
	ErrNoProvidersAvailable = errors.New("no payment providers available")
	ErrPaymentFailed        = errors.New("payment failed on all providers")
	ErrProviderNotFound     = errors.New("provider not found")
	ErrUnsupportedCurrency  = errors.New("unsupported currency for provider")
)

// Provider represents a payment provider type
type Provider string

const (
	ProviderStripe Provider = "stripe"
	ProviderPayPal Provider = "paypal"
)

// PaymentRequest represents a unified payment request
type PaymentRequest struct {
	Amount         int64             // Amount in cents
	Currency       string
	Source         string            // Token or payment source ID
	Description    string
	IdempotencyKey string
	Metadata       map[string]string
	PreferredProvider Provider         // Optional preferred provider
}

// PaymentResult represents a unified payment result
type PaymentResult struct {
	Provider      Provider
	ProviderRef   string
	Status        string
	Amount        int64
	Currency      string
	FailureReason string
}

// RefundRequest represents a unified refund request
type RefundRequest struct {
	Provider    Provider
	ProviderRef string
	Amount      int64  // 0 for full refund
	Currency    string // Required for partial refunds (e.g., "USD", "EUR")
	Reason      string
}

// RefundResult represents a unified refund result
type RefundResult struct {
	Provider  Provider
	RefundRef string
	Amount    int64
	Status    string
}

// ProviderConfig holds configuration for a payment provider
type ProviderConfig struct {
	Provider      Provider
	Priority      int      // Lower is higher priority
	Enabled       bool
	Currencies    []string // Supported currencies
	MaxRetries    int
	RetryDelayMs  int
}

// Router handles payment routing and failover
type Router struct {
	mu sync.RWMutex

	stripeClient *stripe.Client
	paypalClient *paypal.Client

	providers map[Provider]*ProviderConfig
	order     []Provider // Providers ordered by priority
}

// NewRouter creates a new payment router
func NewRouter() *Router {
	return &Router{
		providers: make(map[Provider]*ProviderConfig),
		order:     []Provider{},
	}
}

// RegisterStripe registers the Stripe provider
func (r *Router) RegisterStripe(apiKey string, config *ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stripeClient = stripe.NewClient(apiKey)
	config.Provider = ProviderStripe
	r.providers[ProviderStripe] = config
	r.rebuildOrder()
}

// RegisterPayPal registers the PayPal provider
func (r *Router) RegisterPayPal(clientID, clientSecret string, sandbox bool, config *ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.paypalClient = paypal.NewClient(clientID, clientSecret, sandbox)
	config.Provider = ProviderPayPal
	r.providers[ProviderPayPal] = config
	r.rebuildOrder()
}

// rebuildOrder rebuilds the provider priority order
func (r *Router) rebuildOrder() {
	r.order = make([]Provider, 0, len(r.providers))
	for provider, config := range r.providers {
		if config.Enabled {
			r.order = append(r.order, provider)
		}
	}

	// Sort by priority
	for i := 0; i < len(r.order)-1; i++ {
		for j := i + 1; j < len(r.order); j++ {
			if r.providers[r.order[j]].Priority < r.providers[r.order[i]].Priority {
				r.order[i], r.order[j] = r.order[j], r.order[i]
			}
		}
	}
}

// ProcessPayment processes a payment through available providers
func (r *Router) ProcessPayment(req *PaymentRequest) (*PaymentResult, error) {
	r.mu.RLock()
	providers := r.getProvidersForRequest(req)
	// Copy config data we need while holding the lock
	providerConfigs := make(map[Provider]ProviderConfig)
	for _, p := range providers {
		if cfg, ok := r.providers[p]; ok {
			providerConfigs[p] = *cfg
		}
	}
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, ErrNoProvidersAvailable
	}

	var lastErr error
	for _, provider := range providers {
		config := providerConfigs[provider]
		result, err := r.processWithProvider(provider, &config, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, lastErr)
}

// getProvidersForRequest returns providers that can handle the request
func (r *Router) getProvidersForRequest(req *PaymentRequest) []Provider {
	var result []Provider

	// If preferred provider specified and available, put it first
	if req.PreferredProvider != "" {
		if config, ok := r.providers[req.PreferredProvider]; ok && config.Enabled {
			if r.supportsCurrency(config, req.Currency) {
				result = append(result, req.PreferredProvider)
			}
		}
	}

	// Add other providers in priority order
	for _, provider := range r.order {
		if provider == req.PreferredProvider {
			continue // Already added
		}
		config := r.providers[provider]
		if r.supportsCurrency(config, req.Currency) {
			result = append(result, provider)
		}
	}

	return result
}

// supportsCurrency checks if a provider supports a currency
func (r *Router) supportsCurrency(config *ProviderConfig, currency string) bool {
	if len(config.Currencies) == 0 {
		return true // No restriction
	}
	for _, c := range config.Currencies {
		if c == currency {
			return true
		}
	}
	return false
}

// processWithProvider processes payment with a specific provider
func (r *Router) processWithProvider(provider Provider, config *ProviderConfig, req *PaymentRequest) (*PaymentResult, error) {
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(config.RetryDelayMs) * time.Millisecond)
		}

		result, err := r.executePayment(provider, req)
		if err == nil {
			return result, nil
		}

		// Don't retry on client errors (4xx equivalent)
		if isClientError(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, lastErr
}

// isClientError checks if the error is a client error that shouldn't be retried
func isClientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Common client error patterns - don't retry these
	clientErrors := []string{
		"invalid", "unauthorized", "forbidden", "not found",
		"bad request", "validation", "insufficient funds",
	}
	for _, ce := range clientErrors {
		if contains(errStr, ce) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchLower(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchLower(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// executePayment executes payment on a specific provider
func (r *Router) executePayment(provider Provider, req *PaymentRequest) (*PaymentResult, error) {
	switch provider {
	case ProviderStripe:
		return r.executeStripePayment(req)
	case ProviderPayPal:
		return r.executePayPalPayment(req)
	default:
		return nil, ErrProviderNotFound
	}
}

// executeStripePayment executes a Stripe payment
func (r *Router) executeStripePayment(req *PaymentRequest) (*PaymentResult, error) {
	if r.stripeClient == nil {
		return nil, ErrProviderNotFound
	}

	chargeReq := &stripe.ChargeRequest{
		Amount:         req.Amount,
		Currency:       req.Currency,
		Source:         req.Source,
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	}

	resp, err := r.stripeClient.Charge(chargeReq)
	if err != nil {
		return nil, err
	}

	return &PaymentResult{
		Provider:    ProviderStripe,
		ProviderRef: resp.ID,
		Status:      resp.Status,
		Amount:      resp.Amount,
		Currency:    resp.Currency,
	}, nil
}

// executePayPalPayment executes a PayPal payment
func (r *Router) executePayPalPayment(req *PaymentRequest) (*PaymentResult, error) {
	if r.paypalClient == nil {
		return nil, ErrProviderNotFound
	}

	// Convert cents to decimal string
	amountStr := fmt.Sprintf("%.2f", float64(req.Amount)/100)

	orderReq := &paypal.OrderRequest{
		Intent: "CAPTURE",
		PurchaseUnits: []paypal.PurchaseUnit{
			{
				Amount: paypal.Amount{
					CurrencyCode: req.Currency,
					Value:        amountStr,
				},
				Description: req.Description,
			},
		},
	}

	resp, err := r.paypalClient.CreateOrder(orderReq)
	if err != nil {
		return nil, err
	}

	return &PaymentResult{
		Provider:    ProviderPayPal,
		ProviderRef: resp.ID,
		Status:      resp.Status,
		Amount:      req.Amount,
		Currency:    req.Currency,
	}, nil
}

// ProcessRefund processes a refund through the appropriate provider
func (r *Router) ProcessRefund(req *RefundRequest) (*RefundResult, error) {
	switch req.Provider {
	case ProviderStripe:
		return r.executeStripeRefund(req)
	case ProviderPayPal:
		return r.executePayPalRefund(req)
	default:
		return nil, ErrProviderNotFound
	}
}

// executeStripeRefund executes a Stripe refund
func (r *Router) executeStripeRefund(req *RefundRequest) (*RefundResult, error) {
	if r.stripeClient == nil {
		return nil, ErrProviderNotFound
	}

	refundReq := &stripe.RefundRequest{
		ChargeID: req.ProviderRef,
		Amount:   req.Amount,
		Reason:   req.Reason,
	}

	resp, err := r.stripeClient.Refund(refundReq)
	if err != nil {
		return nil, err
	}

	return &RefundResult{
		Provider:  ProviderStripe,
		RefundRef: resp.ID,
		Amount:    resp.Amount,
		Status:    resp.Status,
	}, nil
}

// executePayPalRefund executes a PayPal refund
func (r *Router) executePayPalRefund(req *RefundRequest) (*RefundResult, error) {
	if r.paypalClient == nil {
		return nil, ErrProviderNotFound
	}

	var refundReq *paypal.RefundRequest
	if req.Amount > 0 {
		// Partial refund requires amount AND currency
		if req.Currency == "" {
			return nil, errors.New("currency is required for partial refunds")
		}
		amountStr := fmt.Sprintf("%.2f", float64(req.Amount)/100)
		refundReq = &paypal.RefundRequest{
			Amount: &paypal.Amount{
				CurrencyCode: req.Currency,
				Value:        amountStr,
			},
			NoteToPayer: req.Reason,
		}
	}

	resp, err := r.paypalClient.RefundCapture(req.ProviderRef, refundReq)
	if err != nil {
		return nil, err
	}

	return &RefundResult{
		Provider:  ProviderPayPal,
		RefundRef: resp.ID,
		Status:    resp.Status,
	}, nil
}

// Healthcheck verifies all registered providers are healthy
func (r *Router) Healthcheck() map[Provider]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[Provider]error)

	if r.stripeClient != nil {
		results[ProviderStripe] = r.stripeClient.Healthcheck()
	}
	if r.paypalClient != nil {
		results[ProviderPayPal] = r.paypalClient.Healthcheck()
	}

	return results
}

// GetProviderStatus returns the status of all providers
func (r *Router) GetProviderStatus() map[Provider]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[Provider]bool)
	for provider, config := range r.providers {
		status[provider] = config.Enabled
	}
	return status
}

// EnableProvider enables a provider
func (r *Router) EnableProvider(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, ok := r.providers[provider]
	if !ok {
		return ErrProviderNotFound
	}
	config.Enabled = true
	r.rebuildOrder()
	return nil
}

// DisableProvider disables a provider
func (r *Router) DisableProvider(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, ok := r.providers[provider]
	if !ok {
		return ErrProviderNotFound
	}
	config.Enabled = false
	r.rebuildOrder()
	return nil
}
