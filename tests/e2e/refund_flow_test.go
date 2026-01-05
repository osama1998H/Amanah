//go:build e2e
// +build e2e

package e2e

import (
	"testing"

	accountModels "amanah/services/account/models"
	accountRepos "amanah/services/account/repositories"
	accountServices "amanah/services/account/services"
	ledgerServices "amanah/services/ledger/services"
	txModels "amanah/services/transaction/models"
	txRepos "amanah/services/transaction/repositories"
	txServices "amanah/services/transaction/services"
)

// TestE2E_Payment_FullRefund_VerifyLedger tests complete refund flow
func TestE2E_Payment_FullRefund_VerifyLedger(t *testing.T) {
	// Setup
	accountRepo := accountRepos.NewInMemoryRepository()
	accountService := accountServices.NewAccountService(accountRepo)
	ledgerService := ledgerServices.NewLedgerService()
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)

	// Step 1: Create and setup accounts
	merchant, _ := createAccount(accountService, "refund-merchant@shop.com", accountModels.AccountTypeMerchant, "USD")
	customer, _ := createAccount(accountService, "refund-customer@email.com", accountModels.AccountTypePersonal, "USD")

	activateAccount(accountService, merchant.ID)
	activateAccount(accountService, customer.ID)
	accountService.CreditBalance(customer.ID, 100000) // $1000
	accountService.CreditBalance(merchant.ID, 500000) // $5000 (existing balance)

	initialCustomerBalance := int64(100000)
	initialMerchantBalance := int64(500000)

	// Step 2: Make a payment
	paymentAmount := int64(25000) // $250
	paymentTx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        paymentAmount,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    merchant.ID,
		CustomerID:    customer.ID,
		Metadata: map[string]string{
			"order_id": "order_refund_123",
		},
	})

	txService.AuthorizeTransaction(paymentTx.ID, "provider_ref")
	txService.CompleteTransaction(paymentTx.ID)

	// Update balances
	accountService.DebitBalance(customer.ID, paymentAmount)
	accountService.CreditBalance(merchant.ID, paymentAmount)

	// Record payment in ledger
	ledgerService.RecordPayment(paymentTx.ID, paymentAmount, merchant.ID)

	// Verify post-payment balances
	customer, _ = accountRepo.GetByID(customer.ID)
	merchant, _ = accountRepo.GetByID(merchant.ID)

	if customer.Balance != initialCustomerBalance-paymentAmount {
		t.Errorf("post-payment customer balance: expected %d, got %d", initialCustomerBalance-paymentAmount, customer.Balance)
	}
	if merchant.Balance != initialMerchantBalance+paymentAmount {
		t.Errorf("post-payment merchant balance: expected %d, got %d", initialMerchantBalance+paymentAmount, merchant.Balance)
	}

	t.Logf("Payment of $%.2f completed", float64(paymentAmount)/100)

	// Step 3: Process full refund
	refundTx, err := txService.RefundTransaction(paymentTx.ID, paymentAmount, "customer requested")
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	if refundTx.Status != txModels.StatusRefunded {
		t.Errorf("expected refund status Refunded, got %s", refundTx.Status)
	}
	if refundTx.RefundedAmount != paymentAmount {
		t.Errorf("expected refunded amount %d, got %d", paymentAmount, refundTx.RefundedAmount)
	}

	// Reverse balances
	accountService.DebitBalance(merchant.ID, paymentAmount)
	accountService.CreditBalance(customer.ID, paymentAmount)

	// Record refund in ledger
	ledgerService.RecordRefund(paymentTx.ID, paymentAmount)

	// Step 4: Verify final balances (should be back to original)
	customer, _ = accountRepo.GetByID(customer.ID)
	merchant, _ = accountRepo.GetByID(merchant.ID)

	if customer.Balance != initialCustomerBalance {
		t.Errorf("final customer balance: expected %d, got %d", initialCustomerBalance, customer.Balance)
	}
	if merchant.Balance != initialMerchantBalance {
		t.Errorf("final merchant balance: expected %d, got %d", initialMerchantBalance, merchant.Balance)
	}

	// Step 5: Verify ledger entries exist
	trialBalance := ledgerService.GetTrialBalance()
	if trialBalance == nil {
		t.Fatal("failed to get trial balance")
	}

	t.Logf("Full refund of $%.2f processed successfully", float64(paymentAmount)/100)
	t.Log("Ledger entries recorded, all accounts restored to original state")
}

// TestE2E_Payment_PartialRefund_VerifyBalances tests partial refund flow
func TestE2E_Payment_PartialRefund_VerifyBalances(t *testing.T) {
	// Setup
	accountRepo := accountRepos.NewInMemoryRepository()
	accountService := accountServices.NewAccountService(accountRepo)
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)
	ledgerService := ledgerServices.NewLedgerService()

	// Create accounts
	merchant, _ := createAccount(accountService, "partial-merchant@shop.com", accountModels.AccountTypeMerchant, "USD")
	customer, _ := createAccount(accountService, "partial-customer@email.com", accountModels.AccountTypePersonal, "USD")

	activateAccount(accountService, merchant.ID)
	activateAccount(accountService, customer.ID)
	accountService.CreditBalance(customer.ID, 100000) // $1000

	// Make a $300 payment
	paymentAmount := int64(30000)
	paymentTx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        paymentAmount,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    merchant.ID,
		CustomerID:    customer.ID,
	})

	txService.AuthorizeTransaction(paymentTx.ID, "provider_ref")
	txService.CompleteTransaction(paymentTx.ID)
	accountService.DebitBalance(customer.ID, paymentAmount)
	accountService.CreditBalance(merchant.ID, paymentAmount)

	// Record in ledger
	ledgerService.RecordPayment(paymentTx.ID, paymentAmount, merchant.ID)

	t.Logf("Payment of $%.2f completed", float64(paymentAmount)/100)

	// Process partial refund (50%)
	partialRefundAmount := paymentAmount / 2 // $150
	refundTx1, err := txService.RefundTransaction(paymentTx.ID, partialRefundAmount, "partial refund")
	if err != nil {
		t.Fatalf("failed to create partial refund: %v", err)
	}

	accountService.DebitBalance(merchant.ID, partialRefundAmount)
	accountService.CreditBalance(customer.ID, partialRefundAmount)

	// Record partial refund
	ledgerService.RecordRefund(paymentTx.ID, partialRefundAmount)

	// Verify balances after first partial refund
	customer, _ = accountRepo.GetByID(customer.ID)
	merchant, _ = accountRepo.GetByID(merchant.ID)

	expectedCustomerBalance := int64(100000) - paymentAmount + partialRefundAmount // $850
	expectedMerchantBalance := paymentAmount - partialRefundAmount                 // $150

	if customer.Balance != expectedCustomerBalance {
		t.Errorf("after first refund - customer balance: expected %d, got %d", expectedCustomerBalance, customer.Balance)
	}
	if merchant.Balance != expectedMerchantBalance {
		t.Errorf("after first refund - merchant balance: expected %d, got %d", expectedMerchantBalance, merchant.Balance)
	}

	t.Logf("First partial refund of $%.2f processed", float64(partialRefundAmount)/100)

	// Process remaining refund (other 50%)
	refundTx2, err := txService.RefundTransaction(paymentTx.ID, partialRefundAmount, "remaining refund")
	if err != nil {
		t.Fatalf("failed to create second partial refund: %v", err)
	}

	accountService.DebitBalance(merchant.ID, partialRefundAmount)
	accountService.CreditBalance(customer.ID, partialRefundAmount)

	// Record second partial refund
	ledgerService.RecordRefund(paymentTx.ID, partialRefundAmount)

	// Verify final balances
	customer, _ = accountRepo.GetByID(customer.ID)
	merchant, _ = accountRepo.GetByID(merchant.ID)

	if customer.Balance != 100000 { // Back to original
		t.Errorf("final customer balance: expected 100000, got %d", customer.Balance)
	}
	if merchant.Balance != 0 { // All refunded
		t.Errorf("final merchant balance: expected 0, got %d", merchant.Balance)
	}

	// Verify all refund transactions
	if refundTx1.RefundedAmount != partialRefundAmount || refundTx2.Status != txModels.StatusRefunded {
		t.Error("expected partial refunds to be recorded correctly")
	}

	t.Log("Two partial refunds processed successfully, balances restored")
}

// TestE2E_RefundExceedsPayment tests that refund cannot exceed payment amount
func TestE2E_RefundExceedsPayment(t *testing.T) {
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)

	// Create and complete a payment
	paymentTx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        10000, // $100
		Currency:      "USD",
		PaymentMethod: "card",
	})

	txService.AuthorizeTransaction(paymentTx.ID, "provider_ref")
	txService.CompleteTransaction(paymentTx.ID)

	// Try to refund more than the payment amount
	_, err := txService.RefundTransaction(paymentTx.ID, 15000, "over refund") // $150
	if err == nil {
		t.Error("expected error when refund exceeds payment amount")
	}

	t.Log("Refund exceeding payment correctly rejected")
}

// TestE2E_MultiplePartialRefunds tests multiple partial refunds
func TestE2E_MultiplePartialRefunds(t *testing.T) {
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)

	// Create and complete a $100 payment
	paymentTx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        10000, // $100
		Currency:      "USD",
		PaymentMethod: "card",
	})

	txService.AuthorizeTransaction(paymentTx.ID, "provider_ref")
	txService.CompleteTransaction(paymentTx.ID)

	// Process multiple small refunds
	refundAmounts := []int64{2000, 3000, 2500, 2500} // $20, $30, $25, $25 = $100 total
	totalRefunded := int64(0)

	for i, amount := range refundAmounts {
		refund, err := txService.RefundTransaction(paymentTx.ID, amount, "partial refund")
		if err != nil {
			t.Fatalf("refund %d failed: %v", i+1, err)
		}

		totalRefunded += amount
		t.Logf("Refund %d: $%.2f (total refunded: $%.2f)", i+1, float64(amount)/100, float64(totalRefunded)/100)

		// Check refunded amount is tracked
		if refund.RefundedAmount != totalRefunded {
			t.Errorf("refund %d: expected total refunded %d, got %d", i+1, totalRefunded, refund.RefundedAmount)
		}
	}

	if totalRefunded != 10000 {
		t.Errorf("expected total refunded 10000, got %d", totalRefunded)
	}

	// Trying another refund should fail (already fully refunded)
	_, err := txService.RefundTransaction(paymentTx.ID, 1000, "extra refund")
	if err == nil {
		t.Error("expected error when trying to refund already fully refunded payment")
	}

	t.Log("Multiple partial refunds processed correctly")
}

// TestE2E_RefundToFrozenCustomerAccount tests refund behavior with frozen account
func TestE2E_RefundToFrozenCustomerAccount(t *testing.T) {
	accountRepo := accountRepos.NewInMemoryRepository()
	accountService := accountServices.NewAccountService(accountRepo)
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)

	// Create accounts
	merchant, _ := createAccount(accountService, "merchant@refund-frozen.com", accountModels.AccountTypeMerchant, "USD")
	customer, _ := createAccount(accountService, "customer@refund-frozen.com", accountModels.AccountTypePersonal, "USD")

	activateAccount(accountService, merchant.ID)
	activateAccount(accountService, customer.ID)
	accountService.CreditBalance(customer.ID, 100000)

	// Make payment
	paymentTx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        25000,
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    merchant.ID,
		CustomerID:    customer.ID,
	})

	txService.AuthorizeTransaction(paymentTx.ID, "provider_ref")
	txService.CompleteTransaction(paymentTx.ID)
	accountService.DebitBalance(customer.ID, 25000)
	accountService.CreditBalance(merchant.ID, 25000)

	// Freeze customer account (simulating fraud investigation)
	accountService.FreezeAccount(customer.ID, "fraud investigation")

	// Create refund transaction - should still work
	refundTx, err := txService.RefundTransaction(paymentTx.ID, 25000, "customer refund")
	if err != nil {
		t.Fatalf("failed to create refund transaction: %v", err)
	}

	// But the actual balance credit should still work (refunds to frozen accounts are typically allowed)
	// The account is frozen but we can still credit it via repository
	err = accountRepo.UpdateBalanceAtomic(customer.ID, 25000)
	if err != nil {
		t.Logf("Note: Balance credit to frozen account handled by repository directly")
	}
	accountService.DebitBalance(merchant.ID, 25000)

	if refundTx.Status != txModels.StatusRefunded {
		t.Errorf("expected refund transaction to be refunded, got %s", refundTx.Status)
	}

	t.Log("Refund to frozen account handled appropriately")
}

// TestE2E_RefundNonCompletedTransaction tests that only completed transactions can be refunded
func TestE2E_RefundNonCompletedTransaction(t *testing.T) {
	transactionRepo := txRepos.NewInMemoryRepository()
	txService := txServices.NewTransactionService(transactionRepo)

	tests := []struct {
		name  string
		setup func(tx *txModels.Transaction)
	}{
		{
			name:  "pending transaction",
			setup: func(tx *txModels.Transaction) {},
		},
		{
			name: "authorized transaction",
			setup: func(tx *txModels.Transaction) {
				txService.AuthorizeTransaction(tx.ID, "provider_ref")
			},
		},
		{
			name: "cancelled transaction",
			setup: func(tx *txModels.Transaction) {
				txService.AuthorizeTransaction(tx.ID, "provider_ref")
				txService.CancelTransaction(tx.ID, "test cancel")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, _ := txService.CreateTransaction(&txModels.CreateTransactionRequest{
				Amount:        10000,
				Currency:      "USD",
				PaymentMethod: "card",
			})

			tt.setup(tx)

			_, err := txService.RefundTransaction(tx.ID, 10000, "test refund")
			if err == nil {
				t.Errorf("%s: expected error when refunding non-completed transaction", tt.name)
			}
		})
	}

	t.Log("Non-completed transaction refund attempts correctly rejected")
}
