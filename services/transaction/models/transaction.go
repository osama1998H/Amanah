package models

import (
	"time"
)

// TransactionStatus represents the state of a transaction
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "pending"
	StatusAuthorized TransactionStatus = "authorized"
	StatusCompleted  TransactionStatus = "completed"
	StatusFailed     TransactionStatus = "failed"
	StatusCancelled  TransactionStatus = "cancelled"
	StatusRefunded   TransactionStatus = "refunded"
)

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[TransactionStatus][]TransactionStatus{
	StatusPending:    {StatusAuthorized, StatusFailed, StatusCancelled},
	StatusAuthorized: {StatusCompleted, StatusFailed, StatusCancelled},
	StatusCompleted:  {StatusRefunded},
	StatusFailed:     {},
	StatusCancelled:  {},
	StatusRefunded:   {},
}

// CanTransitionTo checks if a status transition is valid
func (s TransactionStatus) CanTransitionTo(target TransactionStatus) bool {
	allowed, exists := ValidTransitions[s]
	if !exists {
		return false
	}
	for _, status := range allowed {
		if status == target {
			return true
		}
	}
	return false
}

// Transaction represents a payment transaction
type Transaction struct {
	ID              string            `json:"id"`
	IdempotencyKey  string            `json:"idempotency_key"`
	Amount          int64             `json:"amount"` // Amount in cents
	Currency        string            `json:"currency"`
	Status          TransactionStatus `json:"status"`
	PaymentMethod   string            `json:"payment_method"`
	MerchantID      string            `json:"merchant_id"`
	CustomerID      string            `json:"customer_id"`
	Description     string            `json:"description,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	ProviderRef     string            `json:"provider_ref,omitempty"`
	FailureReason   string            `json:"failure_reason,omitempty"`
	RefundedAmount  int64             `json:"refunded_amount,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
}

// CreateTransactionRequest represents a request to create a transaction
type CreateTransactionRequest struct {
	IdempotencyKey string            `json:"idempotency_key"`
	Amount         int64             `json:"amount"`
	Currency       string            `json:"currency"`
	PaymentMethod  string            `json:"payment_method"`
	MerchantID     string            `json:"merchant_id"`
	CustomerID     string            `json:"customer_id"`
	Description    string            `json:"description,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// RefundRequest represents a request to refund a transaction
type RefundRequest struct {
	Amount int64  `json:"amount,omitempty"` // Partial refund amount, 0 for full
	Reason string `json:"reason,omitempty"`
}

// TransactionResponse wraps a transaction for API responses
type TransactionResponse struct {
	Success     bool         `json:"success"`
	Transaction *Transaction `json:"transaction,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// TransactionListResponse wraps a list of transactions
type TransactionListResponse struct {
	Success      bool           `json:"success"`
	Transactions []*Transaction `json:"transactions"`
	Total        int            `json:"total"`
}
