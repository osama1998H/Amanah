package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"amanah/services/transaction/models"
	"amanah/services/transaction/repositories"
)

var (
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrInvalidCurrency     = errors.New("invalid currency")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrAlreadyRefunded     = errors.New("transaction already refunded")
	ErrRefundExceedsAmount = errors.New("refund amount exceeds transaction amount")
	ErrCannotCancel        = errors.New("cannot cancel transaction in current state")
	ErrCannotRefund        = errors.New("cannot refund transaction in current state")
)

// SupportedCurrencies lists valid currency codes
var SupportedCurrencies = map[string]bool{
	"USD": true,
	"EUR": true,
	"GBP": true,
	"IQD": true,
	"AED": true,
}

// SupportedPaymentMethods lists valid payment methods
var SupportedPaymentMethods = map[string]bool{
	"card":          true,
	"bank_transfer": true,
	"wallet":        true,
	"stripe":        true,
	"paypal":        true,
}

// TransactionService handles transaction business logic
type TransactionService struct {
	repo repositories.TransactionRepository
}

// NewTransactionService creates a new transaction service
func NewTransactionService(repo repositories.TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

// CreateTransaction initiates a new payment transaction
func (s *TransactionService) CreateTransaction(req *models.CreateTransactionRequest) (*models.Transaction, error) {
	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(req.IdempotencyKey)
		if err == nil {
			return existing, nil // Return existing transaction for idempotent request
		}
	}

	// Generate transaction ID
	txID, err := generateID("txn")
	if err != nil {
		return nil, err
	}

	tx := &models.Transaction{
		ID:             txID,
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         models.StatusPending,
		PaymentMethod:  req.PaymentMethod,
		MerchantID:     req.MerchantID,
		CustomerID:     req.CustomerID,
		Description:    req.Description,
		Metadata:       req.Metadata,
	}

	if err := s.repo.Create(tx); err != nil {
		if errors.Is(err, repositories.ErrDuplicateKey) {
			// Handle race condition - return existing
			existing, _ := s.repo.GetByIdempotencyKey(req.IdempotencyKey)
			if existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	return tx, nil
}

// GetTransaction retrieves a transaction by ID
func (s *TransactionService) GetTransaction(id string) (*models.Transaction, error) {
	return s.repo.GetByID(id)
}

// AuthorizeTransaction moves a transaction to authorized state
func (s *TransactionService) AuthorizeTransaction(id string, providerRef string) (*models.Transaction, error) {
	tx, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !tx.Status.CanTransitionTo(models.StatusAuthorized) {
		return nil, repositories.ErrInvalidTransition
	}

	tx.Status = models.StatusAuthorized
	tx.ProviderRef = providerRef

	if err := s.repo.Update(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// CompleteTransaction marks a transaction as completed
func (s *TransactionService) CompleteTransaction(id string) (*models.Transaction, error) {
	tx, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !tx.Status.CanTransitionTo(models.StatusCompleted) {
		return nil, repositories.ErrInvalidTransition
	}

	tx.Status = models.StatusCompleted
	now := time.Now().UTC()
	tx.CompletedAt = &now

	if err := s.repo.Update(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// CancelTransaction cancels a pending or authorized transaction
func (s *TransactionService) CancelTransaction(id string, reason string) (*models.Transaction, error) {
	tx, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !tx.Status.CanTransitionTo(models.StatusCancelled) {
		return nil, ErrCannotCancel
	}

	tx.Status = models.StatusCancelled
	tx.FailureReason = reason

	if err := s.repo.Update(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// FailTransaction marks a transaction as failed
func (s *TransactionService) FailTransaction(id string, reason string) (*models.Transaction, error) {
	tx, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !tx.Status.CanTransitionTo(models.StatusFailed) {
		return nil, repositories.ErrInvalidTransition
	}

	tx.Status = models.StatusFailed
	tx.FailureReason = reason

	if err := s.repo.Update(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// RefundTransaction refunds a completed transaction
func (s *TransactionService) RefundTransaction(id string, amount int64, reason string) (*models.Transaction, error) {
	tx, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !tx.Status.CanTransitionTo(models.StatusRefunded) {
		return nil, ErrCannotRefund
	}

	// Full refund if amount is 0 or equals transaction amount
	refundAmount := amount
	if refundAmount == 0 {
		refundAmount = tx.Amount
	}

	// Check refund doesn't exceed original amount
	if tx.RefundedAmount+refundAmount > tx.Amount {
		return nil, ErrRefundExceedsAmount
	}

	tx.RefundedAmount += refundAmount
	if tx.RefundedAmount >= tx.Amount {
		tx.Status = models.StatusRefunded
	}
	if reason != "" {
		tx.FailureReason = reason
	}

	if err := s.repo.Update(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// ListTransactions returns transactions for a merchant
func (s *TransactionService) ListTransactions(merchantID string, limit, offset int) ([]*models.Transaction, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(merchantID, limit, offset)
}

// validateCreateRequest validates a create transaction request
func (s *TransactionService) validateCreateRequest(req *models.CreateTransactionRequest) error {
	if req.Amount <= 0 {
		return ErrInvalidAmount
	}
	if !SupportedCurrencies[req.Currency] {
		return ErrInvalidCurrency
	}
	if !SupportedPaymentMethods[req.PaymentMethod] {
		return ErrInvalidPaymentMethod
	}
	return nil
}

// generateID creates a unique ID with a prefix
func generateID(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}
