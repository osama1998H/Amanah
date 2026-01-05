package services

import (
	"testing"

	"amanah/services/account/models"
	"amanah/services/account/repositories"
)

// Helper function to create a test service with in-memory repository
func setupTestService() (*AccountService, *repositories.InMemoryRepository) {
	repo := repositories.NewInMemoryRepository()
	service := NewAccountService(repo)
	return service, repo
}

// Helper function to create a valid account request
func validCreateRequest() *models.CreateAccountRequest {
	return &models.CreateAccountRequest{
		Email:    "test@example.com",
		Type:     models.AccountTypePersonal,
		Currency: "USD",
		Profile: &models.Profile{
			FirstName: "John",
			LastName:  "Doe",
		},
	}
}

// ===========================================
// Account Creation Tests
// ===========================================

func TestCreateAccount_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	account, err := service.CreateAccount(req)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}
	if account.Email != req.Email {
		t.Errorf("expected email %s, got %s", req.Email, account.Email)
	}
	if account.Type != req.Type {
		t.Errorf("expected type %s, got %s", req.Type, account.Type)
	}
	if account.Status != models.StatusPending {
		t.Errorf("expected status %s, got %s", models.StatusPending, account.Status)
	}
	if account.KYCStatus != models.KYCNotStarted {
		t.Errorf("expected KYC status %s, got %s", models.KYCNotStarted, account.KYCStatus)
	}
	if account.Balance != 0 {
		t.Errorf("expected balance 0, got %d", account.Balance)
	}
}

func TestCreateAccount_DuplicateEmail(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	// Create first account
	_, err := service.CreateAccount(req)
	if err != nil {
		t.Fatalf("failed to create first account: %v", err)
	}

	// Try to create second account with same email
	_, err = service.CreateAccount(req)
	if err != repositories.ErrEmailAlreadyExists {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestCreateAccount_InvalidEmail(t *testing.T) {
	service, _ := setupTestService()

	tests := []struct {
		name  string
		email string
	}{
		{"empty email", ""},
		{"no domain", "test@"},
		{"no at sign", "testexample.com"},
		{"invalid chars", "test @example.com"},
		{"no tld", "test@example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateRequest()
			req.Email = tt.email

			_, err := service.CreateAccount(req)
			if err != ErrInvalidEmail {
				t.Errorf("expected ErrInvalidEmail for %q, got %v", tt.email, err)
			}
		})
	}
}

func TestCreateAccount_InvalidAccountType(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()
	req.Type = "invalid_type"

	_, err := service.CreateAccount(req)
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestCreateAccount_InvalidCurrency(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()
	req.Currency = "XYZ"

	_, err := service.CreateAccount(req)
	if err != ErrInvalidCurrency {
		t.Errorf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestCreateAccount_AllAccountTypes(t *testing.T) {
	tests := []struct {
		accountType models.AccountType
	}{
		{models.AccountTypePersonal},
		{models.AccountTypeBusiness},
		{models.AccountTypeMerchant},
	}

	for _, tt := range tests {
		t.Run(string(tt.accountType), func(t *testing.T) {
			service, _ := setupTestService()
			req := validCreateRequest()
			req.Type = tt.accountType
			req.Email = string(tt.accountType) + "@example.com"

			account, err := service.CreateAccount(req)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if account.Type != tt.accountType {
				t.Errorf("expected type %s, got %s", tt.accountType, account.Type)
			}
		})
	}
}

func TestCreateAccount_AllSupportedCurrencies(t *testing.T) {
	for currency := range SupportedCurrencies {
		t.Run(currency, func(t *testing.T) {
			service, _ := setupTestService()
			req := validCreateRequest()
			req.Currency = currency
			req.Email = currency + "@example.com"

			account, err := service.CreateAccount(req)
			if err != nil {
				t.Fatalf("expected no error for currency %s, got %v", currency, err)
			}
			if account.Currency != currency {
				t.Errorf("expected currency %s, got %s", currency, account.Currency)
			}
		})
	}
}

// ===========================================
// Account Retrieval Tests
// ===========================================

func TestGetAccount_Found(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)

	account, err := service.GetAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, account.ID)
	}
}

func TestGetAccount_NotFound(t *testing.T) {
	service, _ := setupTestService()

	_, err := service.GetAccount("nonexistent")
	if err != repositories.ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestGetAccountByEmail_Found(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	_, _ = service.CreateAccount(req)

	account, err := service.GetAccountByEmail(req.Email)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Email != req.Email {
		t.Errorf("expected email %s, got %s", req.Email, account.Email)
	}
}

func TestGetAccountByEmail_NotFound(t *testing.T) {
	service, _ := setupTestService()

	_, err := service.GetAccountByEmail("notfound@example.com")
	if err != repositories.ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

// ===========================================
// Account State Transition Tests
// ===========================================

func TestActivateAccount_FromPending(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)

	account, err := service.ActivateAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusActive {
		t.Errorf("expected status %s, got %s", models.StatusActive, account.Status)
	}
}

func TestActivateAccount_AlreadyActive(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	// Activating an already active account should still work
	account, err := service.ActivateAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusActive {
		t.Errorf("expected status %s, got %s", models.StatusActive, account.Status)
	}
}

func TestActivateAccount_Closed(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.CloseAccount(created.ID)

	_, err := service.ActivateAccount(created.ID)
	if err != ErrAccountClosed {
		t.Errorf("expected ErrAccountClosed, got %v", err)
	}
}

func TestFreezeAccount_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	account, err := service.FreezeAccount(created.ID, "suspicious activity")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusFrozen {
		t.Errorf("expected status %s, got %s", models.StatusFrozen, account.Status)
	}
	if account.Metadata["freeze_reason"] != "suspicious activity" {
		t.Errorf("expected freeze_reason in metadata")
	}
}

func TestFreezeAccount_AlreadyFrozen(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.FreezeAccount(created.ID, "first reason")

	// Freezing again should update the reason
	account, err := service.FreezeAccount(created.ID, "second reason")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Metadata["freeze_reason"] != "second reason" {
		t.Errorf("expected updated freeze_reason")
	}
}

func TestUnfreezeAccount_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.FreezeAccount(created.ID, "test")

	account, err := service.UnfreezeAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusActive {
		t.Errorf("expected status %s, got %s", models.StatusActive, account.Status)
	}
	if _, exists := account.Metadata["freeze_reason"]; exists {
		t.Error("expected freeze_reason to be removed from metadata")
	}
}

func TestUnfreezeAccount_NotFrozen(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	// Unfreezing a non-frozen account should return it unchanged
	account, err := service.UnfreezeAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusActive {
		t.Errorf("expected status %s, got %s", models.StatusActive, account.Status)
	}
}

func TestCloseAccount_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)

	account, err := service.CloseAccount(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Status != models.StatusClosed {
		t.Errorf("expected status %s, got %s", models.StatusClosed, account.Status)
	}
}

func TestCloseAccount_NotFound(t *testing.T) {
	service, _ := setupTestService()

	_, err := service.CloseAccount("nonexistent")
	if err != repositories.ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

// ===========================================
// KYC Tests
// ===========================================

func TestUpdateKYCStatus_Verified(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)

	account, err := service.UpdateKYCStatus(created.ID, models.KYCVerified)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.KYCStatus != models.KYCVerified {
		t.Errorf("expected KYC status %s, got %s", models.KYCVerified, account.KYCStatus)
	}
	if account.VerifiedAt == nil {
		t.Error("expected VerifiedAt to be set")
	}
}

func TestUpdateKYCStatus_Rejected(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)

	account, err := service.UpdateKYCStatus(created.ID, models.KYCRejected)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.KYCStatus != models.KYCRejected {
		t.Errorf("expected KYC status %s, got %s", models.KYCRejected, account.KYCStatus)
	}
	if account.VerifiedAt != nil {
		t.Error("expected VerifiedAt to be nil for rejected status")
	}
}

func TestUpdateKYCStatus_AllStatuses(t *testing.T) {
	statuses := []models.KYCStatus{
		models.KYCNotStarted,
		models.KYCPending,
		models.KYCVerified,
		models.KYCRejected,
		models.KYCExpired,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			service, _ := setupTestService()
			req := validCreateRequest()
			req.Email = string(status) + "@example.com"

			created, _ := service.CreateAccount(req)

			account, err := service.UpdateKYCStatus(created.ID, status)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if account.KYCStatus != status {
				t.Errorf("expected KYC status %s, got %s", status, account.KYCStatus)
			}
		})
	}
}

// ===========================================
// Balance Tests
// ===========================================

func TestCreditBalance_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	account, err := service.CreditBalance(created.ID, 10000) // $100.00
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Balance != 10000 {
		t.Errorf("expected balance 10000, got %d", account.Balance)
	}
}

func TestCreditBalance_MultipleCredits(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	service.CreditBalance(created.ID, 5000)
	service.CreditBalance(created.ID, 3000)
	account, _ := service.CreditBalance(created.ID, 2000)

	if account.Balance != 10000 {
		t.Errorf("expected balance 10000, got %d", account.Balance)
	}
}

func TestCreditBalance_InvalidAmount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	tests := []struct {
		name   string
		amount int64
	}{
		{"zero", 0},
		{"negative", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreditBalance(created.ID, tt.amount)
			if err != ErrInvalidAmount {
				t.Errorf("expected ErrInvalidAmount for amount %d, got %v", tt.amount, err)
			}
		})
	}
}

func TestCreditBalance_FrozenAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.FreezeAccount(created.ID, "test")

	_, err := service.CreditBalance(created.ID, 10000)
	if err != ErrAccountFrozen {
		t.Errorf("expected ErrAccountFrozen, got %v", err)
	}
}

func TestCreditBalance_ClosedAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.CloseAccount(created.ID)

	_, err := service.CreditBalance(created.ID, 10000)
	if err != ErrAccountClosed {
		t.Errorf("expected ErrAccountClosed, got %v", err)
	}
}

func TestDebitBalance_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.CreditBalance(created.ID, 10000)

	account, err := service.DebitBalance(created.ID, 3000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Balance != 7000 {
		t.Errorf("expected balance 7000, got %d", account.Balance)
	}
}

func TestDebitBalance_InsufficientFunds(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.CreditBalance(created.ID, 5000)

	_, err := service.DebitBalance(created.ID, 10000)
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestDebitBalance_ExactBalance(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.CreditBalance(created.ID, 5000)

	account, err := service.DebitBalance(created.ID, 5000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Balance != 0 {
		t.Errorf("expected balance 0, got %d", account.Balance)
	}
}

func TestDebitBalance_FrozenAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.CreditBalance(created.ID, 10000)
	service.FreezeAccount(created.ID, "test")

	_, err := service.DebitBalance(created.ID, 5000)
	if err != ErrAccountFrozen {
		t.Errorf("expected ErrAccountFrozen, got %v", err)
	}
}

// ===========================================
// Payment Instrument Tests
// ===========================================

func TestAddPaymentInstrument_Card(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	piReq := &models.AddPaymentInstrumentRequest{
		Type:      "card",
		Token:     "tok_visa",
		IsDefault: true,
	}

	account, err := service.AddPaymentInstrument(created.ID, piReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(account.PaymentInstruments) != 1 {
		t.Errorf("expected 1 payment instrument, got %d", len(account.PaymentInstruments))
	}
	if account.PaymentInstruments[0].Type != "card" {
		t.Errorf("expected type 'card', got %s", account.PaymentInstruments[0].Type)
	}
	if !account.PaymentInstruments[0].IsDefault {
		t.Error("expected payment instrument to be default")
	}
}

func TestAddPaymentInstrument_BankAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	piReq := &models.AddPaymentInstrumentRequest{
		Type:  "bank_account",
		Token: "ba_test",
	}

	account, err := service.AddPaymentInstrument(created.ID, piReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.PaymentInstruments[0].Type != "bank_account" {
		t.Errorf("expected type 'bank_account', got %s", account.PaymentInstruments[0].Type)
	}
}

func TestAddPaymentInstrument_DefaultOverride(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	// Add first card as default
	piReq1 := &models.AddPaymentInstrumentRequest{
		Type:      "card",
		Token:     "tok_visa",
		IsDefault: true,
	}
	service.AddPaymentInstrument(created.ID, piReq1)

	// Add second card as new default
	piReq2 := &models.AddPaymentInstrumentRequest{
		Type:      "card",
		Token:     "tok_mastercard",
		IsDefault: true,
	}
	account, _ := service.AddPaymentInstrument(created.ID, piReq2)

	// Only second should be default
	defaultCount := 0
	for _, pi := range account.PaymentInstruments {
		if pi.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("expected 1 default payment instrument, got %d", defaultCount)
	}
}

func TestAddPaymentInstrument_FrozenAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)
	service.FreezeAccount(created.ID, "test")

	piReq := &models.AddPaymentInstrumentRequest{
		Type:  "card",
		Token: "tok_visa",
	}

	_, err := service.AddPaymentInstrument(created.ID, piReq)
	if err != ErrAccountFrozen {
		t.Errorf("expected ErrAccountFrozen, got %v", err)
	}
}

func TestRemovePaymentInstrument_Success(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	piReq := &models.AddPaymentInstrumentRequest{Type: "card", Token: "tok_visa"}
	account, _ := service.AddPaymentInstrument(created.ID, piReq)

	instrumentID := account.PaymentInstruments[0].ID
	account, err := service.RemovePaymentInstrument(created.ID, instrumentID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(account.PaymentInstruments) != 0 {
		t.Errorf("expected 0 payment instruments, got %d", len(account.PaymentInstruments))
	}
}

func TestRemovePaymentInstrument_NotFound(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	_, err := service.RemovePaymentInstrument(created.ID, "nonexistent")
	if err != repositories.ErrInstrumentNotFound {
		t.Errorf("expected ErrInstrumentNotFound, got %v", err)
	}
}

// ===========================================
// Update Account Tests
// ===========================================

func TestUpdateAccount_Profile(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	updateReq := &models.UpdateAccountRequest{
		Profile: &models.Profile{
			FirstName:   "Jane",
			LastName:    "Smith",
			Phone:       "+1234567890",
			DateOfBirth: "1990-01-01",
		},
	}

	account, err := service.UpdateAccount(created.ID, updateReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Profile.FirstName != "Jane" {
		t.Errorf("expected FirstName 'Jane', got %s", account.Profile.FirstName)
	}
	if account.Profile.Phone != "+1234567890" {
		t.Errorf("expected Phone '+1234567890', got %s", account.Profile.Phone)
	}
}

func TestUpdateAccount_Metadata(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.ActivateAccount(created.ID)

	updateReq := &models.UpdateAccountRequest{
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	account, err := service.UpdateAccount(created.ID, updateReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if account.Metadata["key1"] != "value1" {
		t.Errorf("expected metadata key1='value1', got %s", account.Metadata["key1"])
	}
}

func TestUpdateAccount_FrozenAccount(t *testing.T) {
	service, _ := setupTestService()
	req := validCreateRequest()

	created, _ := service.CreateAccount(req)
	service.FreezeAccount(created.ID, "test")

	updateReq := &models.UpdateAccountRequest{
		Metadata: map[string]string{"key": "value"},
	}

	_, err := service.UpdateAccount(created.ID, updateReq)
	if err != ErrAccountFrozen {
		t.Errorf("expected ErrAccountFrozen, got %v", err)
	}
}

// ===========================================
// List Accounts Tests
// ===========================================

func TestListAccounts_Empty(t *testing.T) {
	service, _ := setupTestService()

	accounts, total, err := service.ListAccounts("", 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestListAccounts_WithAccounts(t *testing.T) {
	service, _ := setupTestService()

	// Create multiple accounts
	for i := 0; i < 5; i++ {
		req := validCreateRequest()
		req.Email = string(rune('a'+i)) + "@example.com"
		service.CreateAccount(req)
	}

	accounts, total, err := service.ListAccounts("", 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(accounts) != 5 {
		t.Errorf("expected 5 accounts, got %d", len(accounts))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
}

func TestListAccounts_ByType(t *testing.T) {
	service, _ := setupTestService()

	// Create accounts of different types
	req1 := validCreateRequest()
	req1.Type = models.AccountTypePersonal
	service.CreateAccount(req1)

	req2 := validCreateRequest()
	req2.Email = "business@example.com"
	req2.Type = models.AccountTypeBusiness
	service.CreateAccount(req2)

	req3 := validCreateRequest()
	req3.Email = "merchant@example.com"
	req3.Type = models.AccountTypeMerchant
	service.CreateAccount(req3)

	// Filter by personal type
	accounts, total, err := service.ListAccounts(models.AccountTypePersonal, 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected 1 personal account, got %d", len(accounts))
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestListAccounts_Pagination(t *testing.T) {
	service, _ := setupTestService()

	// Create 10 accounts
	for i := 0; i < 10; i++ {
		req := validCreateRequest()
		req.Email = string(rune('a'+i)) + "@example.com"
		service.CreateAccount(req)
	}

	// Get first page
	accounts, total, _ := service.ListAccounts("", 3, 0)
	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts on first page, got %d", len(accounts))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Get second page
	accounts, _, _ = service.ListAccounts("", 3, 3)
	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts on second page, got %d", len(accounts))
	}
}

func TestListAccounts_InvalidPagination(t *testing.T) {
	service, _ := setupTestService()

	_, _, err := service.ListAccounts("", 10, -1)
	if err != ErrInvalidPagination {
		t.Errorf("expected ErrInvalidPagination for negative offset, got %v", err)
	}
}

func TestListAccounts_DefaultLimit(t *testing.T) {
	service, _ := setupTestService()

	// Create 25 accounts
	for i := 0; i < 25; i++ {
		req := validCreateRequest()
		req.Email = string(rune('a'+i)) + "@example.com"
		service.CreateAccount(req)
	}

	// With limit 0, should use default of 20
	accounts, _, _ := service.ListAccounts("", 0, 0)
	if len(accounts) != 20 {
		t.Errorf("expected default limit of 20, got %d", len(accounts))
	}
}

func TestListAccounts_MaxLimit(t *testing.T) {
	service, _ := setupTestService()

	// Create 150 accounts
	for i := 0; i < 150; i++ {
		req := validCreateRequest()
		req.Email = string(rune('a'+(i%26))) + string(rune('0'+(i/26))) + "@example.com"
		service.CreateAccount(req)
	}

	// With limit > 100, should cap at 100
	accounts, _, _ := service.ListAccounts("", 200, 0)
	if len(accounts) != 100 {
		t.Errorf("expected max limit of 100, got %d", len(accounts))
	}
}
