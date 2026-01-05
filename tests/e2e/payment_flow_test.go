//go:build e2e
// +build e2e

package e2e

import (
	"testing"
	"time"

	accountModels "amanah/services/account/models"
	accountRepos "amanah/services/account/repositories"
	accountServices "amanah/services/account/services"
	ledgerServices "amanah/services/ledger/services"
	txModels "amanah/services/transaction/models"
	txRepos "amanah/services/transaction/repositories"
	txServices "amanah/services/transaction/services"
)

// E2E test helpers
type testEnvironment struct {
	accountService     *accountServices.AccountService
	accountRepo        *accountRepos.InMemoryRepository
	ledgerService      *ledgerServices.LedgerService
	transactionService *txServices.TransactionService
}

func setupTestEnvironment() *testEnvironment {
	accountRepo := accountRepos.NewInMemoryRepository()
	accountService := accountServices.NewAccountService(accountRepo)
	ledgerService := ledgerServices.NewLedgerService()
	transactionRepo := txRepos.NewInMemoryRepository()
	transactionService := txServices.NewTransactionService(transactionRepo)

	return &testEnvironment{
		accountService:     accountService,
		accountRepo:        accountRepo,
		ledgerService:      ledgerService,
		transactionService: transactionService,
	}
}

// createAccount helper creates and returns an account
func createAccount(svc *accountServices.AccountService, email string, accType accountModels.AccountType, currency string) (*accountModels.Account, error) {
	return svc.CreateAccount(&accountModels.CreateAccountRequest{
		Email:    email,
		Type:     accType,
		Currency: currency,
	})
}

// activateAccount helper completes KYC and activates
func activateAccount(svc *accountServices.AccountService, id string) error {
	svc.UpdateKYCStatus(id, accountModels.KYCVerified)
	_, err := svc.ActivateAccount(id)
	return err
}

// TestE2E_CreateAccount_MakePayment_CheckLedger tests the complete payment flow
func TestE2E_CreateAccount_MakePayment_CheckLedger(t *testing.T) {
	env := setupTestEnvironment()

	// Step 1: Create merchant account
	merchantAccount, err := createAccount(env.accountService, "merchant@example.com", accountModels.AccountTypeMerchant, "USD")
	if err != nil {
		t.Fatalf("failed to create merchant account: %v", err)
	}
	t.Logf("Created merchant account: %s", merchantAccount.ID)

	// Step 2: Create customer account
	customerAccount, err := createAccount(env.accountService, "customer@example.com", accountModels.AccountTypePersonal, "USD")
	if err != nil {
		t.Fatalf("failed to create customer account: %v", err)
	}
	t.Logf("Created customer account: %s", customerAccount.ID)

	// Step 3: Activate both accounts
	if err := activateAccount(env.accountService, merchantAccount.ID); err != nil {
		t.Fatalf("failed to activate merchant account: %v", err)
	}
	if err := activateAccount(env.accountService, customerAccount.ID); err != nil {
		t.Fatalf("failed to activate customer account: %v", err)
	}

	// Step 4: Fund customer account (simulate deposit)
	if _, err := env.accountService.CreditBalance(customerAccount.ID, 100000); err != nil { // $1000.00
		t.Fatalf("failed to fund customer account: %v", err)
	}

	// Verify customer balance
	customerAccount, _ = env.accountRepo.GetByID(customerAccount.ID)
	if customerAccount.Balance != 100000 {
		t.Errorf("expected customer balance 100000, got %d", customerAccount.Balance)
	}

	// Step 5: Create a payment transaction
	tx, err := env.transactionService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        25000, // $250.00
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    merchantAccount.ID,
		CustomerID:    customerAccount.ID,
		Description:   "Product purchase",
		Metadata: map[string]string{
			"order_id": "order_123",
		},
	})
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	t.Logf("Created transaction: %s", tx.ID)

	// Step 6: Authorize the transaction
	tx, err = env.transactionService.AuthorizeTransaction(tx.ID, "provider_ref_123")
	if err != nil {
		t.Fatalf("failed to authorize transaction: %v", err)
	}
	if tx.Status != txModels.StatusAuthorized {
		t.Errorf("expected status Authorized, got %s", tx.Status)
	}

	// Step 7: Complete the transaction
	tx, err = env.transactionService.CompleteTransaction(tx.ID)
	if err != nil {
		t.Fatalf("failed to complete transaction: %v", err)
	}
	if tx.Status != txModels.StatusCompleted {
		t.Errorf("expected status Completed, got %s", tx.Status)
	}

	// Step 8: Update account balances (simulate settlement)
	if _, err := env.accountService.DebitBalance(customerAccount.ID, 25000); err != nil {
		t.Fatalf("failed to debit customer account: %v", err)
	}
	if _, err := env.accountService.CreditBalance(merchantAccount.ID, 25000); err != nil {
		t.Fatalf("failed to credit merchant account: %v", err)
	}

	// Step 9: Record in ledger
	journal, err := env.ledgerService.RecordPayment(tx.ID, 25000, merchantAccount.ID)
	if err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}
	t.Logf("Created journal entry: %s", journal.ID)

	// Verify final balances
	customerAccount, _ = env.accountRepo.GetByID(customerAccount.ID)
	merchantAccount, _ = env.accountRepo.GetByID(merchantAccount.ID)

	if customerAccount.Balance != 75000 { // $1000 - $250 = $750
		t.Errorf("expected customer balance 75000, got %d", customerAccount.Balance)
	}
	if merchantAccount.Balance != 25000 { // $250
		t.Errorf("expected merchant balance 25000, got %d", merchantAccount.Balance)
	}

	// Verify transaction record
	finalTx, err := env.transactionService.GetTransaction(tx.ID)
	if err != nil {
		t.Fatalf("failed to get transaction: %v", err)
	}
	if finalTx.Status != txModels.StatusCompleted {
		t.Errorf("expected final transaction status Completed, got %s", finalTx.Status)
	}
	if finalTx.Amount != 25000 {
		t.Errorf("expected transaction amount 25000, got %d", finalTx.Amount)
	}

	t.Log("E2E payment flow completed successfully")
}

// TestE2E_MultiplePayments tests multiple payments in sequence
func TestE2E_MultiplePayments(t *testing.T) {
	env := setupTestEnvironment()

	// Create accounts
	merchant, _ := createAccount(env.accountService, "merchant@shop.com", accountModels.AccountTypeMerchant, "USD")
	customer, _ := createAccount(env.accountService, "buyer@email.com", accountModels.AccountTypePersonal, "USD")

	activateAccount(env.accountService, merchant.ID)
	activateAccount(env.accountService, customer.ID)
	env.accountService.CreditBalance(customer.ID, 500000) // $5000

	// Simulate multiple purchases
	purchases := []int64{10000, 25000, 5000, 15000, 7500} // Various amounts
	totalSpent := int64(0)

	for i, amount := range purchases {
		tx, err := env.transactionService.CreateTransaction(&txModels.CreateTransactionRequest{
			Amount:        amount,
			Currency:      "USD",
			PaymentMethod: "card",
			MerchantID:    merchant.ID,
			CustomerID:    customer.ID,
			Metadata:      map[string]string{"purchase_num": string(rune('1' + i))},
		})
		if err != nil {
			t.Fatalf("purchase %d: failed to create transaction: %v", i+1, err)
		}

		env.transactionService.AuthorizeTransaction(tx.ID, "provider_ref")
		env.transactionService.CompleteTransaction(tx.ID)

		env.accountService.DebitBalance(customer.ID, amount)
		env.accountService.CreditBalance(merchant.ID, amount)
		totalSpent += amount
	}

	// Verify final balances
	customer, _ = env.accountRepo.GetByID(customer.ID)
	merchant, _ = env.accountRepo.GetByID(merchant.ID)

	expectedCustomerBalance := int64(500000) - totalSpent
	if customer.Balance != expectedCustomerBalance {
		t.Errorf("expected customer balance %d, got %d", expectedCustomerBalance, customer.Balance)
	}
	if merchant.Balance != totalSpent {
		t.Errorf("expected merchant balance %d, got %d", totalSpent, merchant.Balance)
	}

	// Verify all transactions
	txs, _, _ := env.transactionService.ListTransactions("", 10, 0)
	if len(txs) != len(purchases) {
		t.Errorf("expected %d transactions, got %d", len(purchases), len(txs))
	}

	for _, tx := range txs {
		if tx.Status != txModels.StatusCompleted {
			t.Errorf("expected all transactions to be completed, got %s", tx.Status)
		}
	}

	t.Logf("Completed %d purchases totaling $%.2f", len(purchases), float64(totalSpent)/100)
}

// TestE2E_ConcurrentPayments tests concurrent payment processing
func TestE2E_ConcurrentPayments(t *testing.T) {
	env := setupTestEnvironment()

	// Create accounts
	merchant, _ := createAccount(env.accountService, "merchant@concurrent.com", accountModels.AccountTypeMerchant, "USD")
	activateAccount(env.accountService, merchant.ID)

	// Create multiple customers
	numCustomers := 10
	customers := make([]*accountModels.Account, numCustomers)
	for i := 0; i < numCustomers; i++ {
		c, _ := createAccount(
			env.accountService,
			string(rune('a'+i))+"@email.com",
			accountModels.AccountTypePersonal,
			"USD",
		)
		activateAccount(env.accountService, c.ID)
		env.accountService.CreditBalance(c.ID, 100000) // $1000 each
		customers[i] = c
	}

	// Simulate concurrent payments
	done := make(chan bool, numCustomers)
	paymentAmount := int64(5000) // $50

	for _, customer := range customers {
		go func(c *accountModels.Account) {
			tx, _ := env.transactionService.CreateTransaction(&txModels.CreateTransactionRequest{
				Amount:        paymentAmount,
				Currency:      "USD",
				PaymentMethod: "card",
				MerchantID:    merchant.ID,
				CustomerID:    c.ID,
			})
			env.transactionService.AuthorizeTransaction(tx.ID, "provider_ref")
			env.transactionService.CompleteTransaction(tx.ID)
			env.accountService.DebitBalance(c.ID, paymentAmount)
			env.accountRepo.UpdateBalanceAtomic(merchant.ID, paymentAmount)
			done <- true
		}(customer)
	}

	// Wait for all payments to complete
	for i := 0; i < numCustomers; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent payments")
		}
	}

	// Verify merchant received all payments
	merchant, _ = env.accountRepo.GetByID(merchant.ID)
	expectedMerchantBalance := paymentAmount * int64(numCustomers)
	if merchant.Balance != expectedMerchantBalance {
		t.Errorf("expected merchant balance %d, got %d", expectedMerchantBalance, merchant.Balance)
	}

	// Verify all customers were debited
	for _, customer := range customers {
		c, _ := env.accountRepo.GetByID(customer.ID)
		expectedBalance := int64(100000) - paymentAmount
		if c.Balance != expectedBalance {
			t.Errorf("customer %s: expected balance %d, got %d", c.Email, expectedBalance, c.Balance)
		}
	}

	t.Logf("Processed %d concurrent payments successfully", numCustomers)
}

// TestE2E_PaymentWithInsufficientFunds tests payment rejection
func TestE2E_PaymentWithInsufficientFunds(t *testing.T) {
	env := setupTestEnvironment()

	// Create and activate accounts
	merchant, _ := createAccount(env.accountService, "merchant@test.com", accountModels.AccountTypeMerchant, "USD")
	customer, _ := createAccount(env.accountService, "poor@email.com", accountModels.AccountTypePersonal, "USD")

	activateAccount(env.accountService, merchant.ID)
	activateAccount(env.accountService, customer.ID)
	env.accountService.CreditBalance(customer.ID, 5000) // Only $50

	// Try to make a $100 payment
	tx, err := env.transactionService.CreateTransaction(&txModels.CreateTransactionRequest{
		Amount:        10000, // $100
		Currency:      "USD",
		PaymentMethod: "card",
		MerchantID:    merchant.ID,
		CustomerID:    customer.ID,
	})
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	tx, _ = env.transactionService.AuthorizeTransaction(tx.ID, "provider_ref")

	// Attempt to debit customer - should fail
	_, err = env.accountService.DebitBalance(customer.ID, 10000)
	if err == nil {
		t.Error("expected error for insufficient funds")
	}

	// Transaction should be cancelled
	tx, _ = env.transactionService.CancelTransaction(tx.ID, "insufficient funds")
	if tx.Status != txModels.StatusCancelled {
		t.Errorf("expected status Cancelled, got %s", tx.Status)
	}

	// Customer balance should remain unchanged
	customer, _ = env.accountRepo.GetByID(customer.ID)
	if customer.Balance != 5000 {
		t.Errorf("expected customer balance 5000, got %d", customer.Balance)
	}

	t.Log("Insufficient funds payment correctly rejected")
}

// TestE2E_FrozenAccountPayment tests that frozen accounts cannot make payments
func TestE2E_FrozenAccountPayment(t *testing.T) {
	env := setupTestEnvironment()

	// Create and activate account
	customer, _ := createAccount(env.accountService, "frozen@email.com", accountModels.AccountTypePersonal, "USD")
	activateAccount(env.accountService, customer.ID)
	env.accountService.CreditBalance(customer.ID, 100000)

	// Freeze the account
	if _, err := env.accountService.FreezeAccount(customer.ID, "suspicious activity"); err != nil {
		t.Fatalf("failed to freeze account: %v", err)
	}

	// Try to debit frozen account - should fail
	_, err := env.accountService.DebitBalance(customer.ID, 10000)
	if err == nil {
		t.Error("expected error when debiting frozen account")
	}

	// Balance should remain unchanged
	customer, _ = env.accountRepo.GetByID(customer.ID)
	if customer.Balance != 100000 {
		t.Errorf("expected balance 100000, got %d", customer.Balance)
	}

	t.Log("Frozen account payment correctly rejected")
}
