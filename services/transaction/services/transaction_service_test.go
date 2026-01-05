package services

import (
	"testing"

	"amanah/services/transaction/models"
	"amanah/services/transaction/repositories"
)

func TestCreateTransaction(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	req := &models.CreateTransactionRequest{
		IdempotencyKey: "test-key-1",
		Amount:         1000,
		Currency:       "USD",
		PaymentMethod:  "card",
		MerchantID:     "merchant-1",
		CustomerID:     "customer-1",
		Description:    "Test transaction",
	}

	tx, err := svc.CreateTransaction(req)
	if err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	if tx.ID == "" {
		t.Error("Expected transaction ID to be set")
	}
	if tx.Status != models.StatusPending {
		t.Errorf("Expected status %s, got %s", models.StatusPending, tx.Status)
	}
	if tx.Amount != 1000 {
		t.Errorf("Expected amount 1000, got %d", tx.Amount)
	}
}

func TestIdempotency(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	req := &models.CreateTransactionRequest{
		IdempotencyKey: "idempotent-key",
		Amount:         2000,
		Currency:       "USD",
		PaymentMethod:  "card",
		MerchantID:     "merchant-1",
		CustomerID:     "customer-1",
	}

	// First request
	tx1, err := svc.CreateTransaction(req)
	if err != nil {
		t.Fatalf("First CreateTransaction failed: %v", err)
	}

	// Second request with same key
	tx2, err := svc.CreateTransaction(req)
	if err != nil {
		t.Fatalf("Second CreateTransaction failed: %v", err)
	}

	if tx1.ID != tx2.ID {
		t.Errorf("Idempotent requests should return same transaction")
	}
}

func TestTransactionStateTransitions(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	// Create transaction
	req := &models.CreateTransactionRequest{
		Amount:        1000,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    "merchant-1",
		CustomerID:    "customer-1",
	}

	tx, _ := svc.CreateTransaction(req)

	// Authorize
	tx, err := svc.AuthorizeTransaction(tx.ID, "provider-ref-123")
	if err != nil {
		t.Fatalf("AuthorizeTransaction failed: %v", err)
	}
	if tx.Status != models.StatusAuthorized {
		t.Errorf("Expected status %s, got %s", models.StatusAuthorized, tx.Status)
	}

	// Complete
	tx, err = svc.CompleteTransaction(tx.ID)
	if err != nil {
		t.Fatalf("CompleteTransaction failed: %v", err)
	}
	if tx.Status != models.StatusCompleted {
		t.Errorf("Expected status %s, got %s", models.StatusCompleted, tx.Status)
	}

	// Refund
	tx, err = svc.RefundTransaction(tx.ID, 500, "Partial refund")
	if err != nil {
		t.Fatalf("RefundTransaction failed: %v", err)
	}
	if tx.RefundedAmount != 500 {
		t.Errorf("Expected refunded amount 500, got %d", tx.RefundedAmount)
	}
}

func TestInvalidTransitions(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	req := &models.CreateTransactionRequest{
		Amount:        1000,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    "merchant-1",
		CustomerID:    "customer-1",
	}

	tx, _ := svc.CreateTransaction(req)

	// Try to complete without authorizing
	_, err := svc.CompleteTransaction(tx.ID)
	if err == nil {
		t.Error("Expected error when completing pending transaction")
	}

	// Try to refund pending transaction
	_, err = svc.RefundTransaction(tx.ID, 0, "")
	if err == nil {
		t.Error("Expected error when refunding pending transaction")
	}
}

func TestValidation(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	tests := []struct {
		name    string
		req     *models.CreateTransactionRequest
		wantErr error
	}{
		{
			name: "invalid amount",
			req: &models.CreateTransactionRequest{
				Amount:        -100,
				Currency:      "USD",
				PaymentMethod: "card",
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "invalid currency",
			req: &models.CreateTransactionRequest{
				Amount:        1000,
				Currency:      "XXX",
				PaymentMethod: "card",
			},
			wantErr: ErrInvalidCurrency,
		},
		{
			name: "invalid payment method",
			req: &models.CreateTransactionRequest{
				Amount:        1000,
				Currency:      "USD",
				PaymentMethod: "invalid",
			},
			wantErr: ErrInvalidPaymentMethod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateTransaction(tt.req)
			if err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCancelTransaction(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	req := &models.CreateTransactionRequest{
		Amount:        1000,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    "merchant-1",
		CustomerID:    "customer-1",
	}

	tx, _ := svc.CreateTransaction(req)

	// Cancel pending transaction
	tx, err := svc.CancelTransaction(tx.ID, "User requested")
	if err != nil {
		t.Fatalf("CancelTransaction failed: %v", err)
	}
	if tx.Status != models.StatusCancelled {
		t.Errorf("Expected status %s, got %s", models.StatusCancelled, tx.Status)
	}
	if tx.FailureReason != "User requested" {
		t.Errorf("Expected failure reason 'User requested', got '%s'", tx.FailureReason)
	}
}

func TestListTransactions(t *testing.T) {
	repo := repositories.NewInMemoryRepository()
	svc := NewTransactionService(repo)

	// Create multiple transactions
	for i := 0; i < 5; i++ {
		req := &models.CreateTransactionRequest{
			Amount:        int64(1000 * (i + 1)),
			Currency:      "USD",
			PaymentMethod: "card",
			MerchantID:    "merchant-1",
			CustomerID:    "customer-1",
		}
		svc.CreateTransaction(req)
	}

	// List all
	txs, total, err := svc.ListTransactions("", 10, 0)
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected 5 transactions, got %d", total)
	}
	if len(txs) != 5 {
		t.Errorf("Expected 5 transactions in result, got %d", len(txs))
	}

	// Test pagination
	txs, _, err = svc.ListTransactions("", 2, 0)
	if err != nil {
		t.Fatalf("ListTransactions with pagination failed: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("Expected 2 transactions with limit, got %d", len(txs))
	}
}
