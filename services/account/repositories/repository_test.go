package repositories

import (
	"sync"
	"testing"

	"amanah/services/account/models"
)

func TestInMemoryRepository_Create(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:       "acc_123",
		Email:    "test@example.com",
		Type:     models.AccountTypePersonal,
		Status:   models.StatusPending,
		Currency: "USD",
	}

	err := repo.Create(account)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify timestamps are set
	if account.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if account.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestInMemoryRepository_Create_DuplicateEmail(t *testing.T) {
	repo := NewInMemoryRepository()

	account1 := &models.Account{
		ID:    "acc_1",
		Email: "test@example.com",
	}
	repo.Create(account1)

	account2 := &models.Account{
		ID:    "acc_2",
		Email: "test@example.com",
	}

	err := repo.Create(account2)
	if err != ErrEmailAlreadyExists {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestInMemoryRepository_GetByID(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "test@example.com",
	}
	repo.Create(account)

	result, err := repo.GetByID("acc_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "acc_123" {
		t.Errorf("expected ID acc_123, got %s", result.ID)
	}
}

func TestInMemoryRepository_GetByID_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	_, err := repo.GetByID("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_GetByEmail(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "test@example.com",
	}
	repo.Create(account)

	result, err := repo.GetByEmail("test@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", result.Email)
	}
}

func TestInMemoryRepository_GetByEmail_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	_, err := repo.GetByEmail("notfound@example.com")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_Update(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:     "acc_123",
		Email:  "test@example.com",
		Status: models.StatusPending,
	}
	repo.Create(account)

	account.Status = models.StatusActive
	err := repo.Update(account)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, _ := repo.GetByID("acc_123")
	if result.Status != models.StatusActive {
		t.Errorf("expected status active, got %s", result.Status)
	}
}

func TestInMemoryRepository_Update_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "nonexistent",
		Email: "test@example.com",
	}

	err := repo.Update(account)
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_Update_EmailChange(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "old@example.com",
	}
	repo.Create(account)

	// Create a new account object with changed email to simulate update
	updatedAccount := &models.Account{
		ID:    "acc_123",
		Email: "new@example.com",
	}
	repo.Update(updatedAccount)

	// Old email should not be found
	_, err := repo.GetByEmail("old@example.com")
	if err != ErrAccountNotFound {
		t.Error("expected old email to not be found")
	}

	// New email should be found
	result, err := repo.GetByEmail("new@example.com")
	if err != nil {
		t.Fatalf("expected new email to be found, got %v", err)
	}
	if result.ID != "acc_123" {
		t.Error("expected correct account for new email")
	}
}

func TestInMemoryRepository_Delete(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "test@example.com",
	}
	repo.Create(account)

	err := repo.Delete("acc_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should not be found by ID
	_, err = repo.GetByID("acc_123")
	if err != ErrAccountNotFound {
		t.Error("expected account to be deleted")
	}

	// Should not be found by email
	_, err = repo.GetByEmail("test@example.com")
	if err != ErrAccountNotFound {
		t.Error("expected email index to be cleared")
	}
}

func TestInMemoryRepository_Delete_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	err := repo.Delete("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_List_Empty(t *testing.T) {
	repo := NewInMemoryRepository()

	accounts, total, err := repo.List("", 10, 0)
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

func TestInMemoryRepository_List_WithAccounts(t *testing.T) {
	repo := NewInMemoryRepository()

	for i := 0; i < 5; i++ {
		account := &models.Account{
			ID:    string(rune('a' + i)),
			Email: string(rune('a'+i)) + "@example.com",
			Type:  models.AccountTypePersonal,
		}
		repo.Create(account)
	}

	accounts, total, err := repo.List("", 10, 0)
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

func TestInMemoryRepository_List_ByType(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create accounts of different types
	personal := &models.Account{ID: "p1", Email: "p@example.com", Type: models.AccountTypePersonal}
	business := &models.Account{ID: "b1", Email: "b@example.com", Type: models.AccountTypeBusiness}
	merchant := &models.Account{ID: "m1", Email: "m@example.com", Type: models.AccountTypeMerchant}

	repo.Create(personal)
	repo.Create(business)
	repo.Create(merchant)

	accounts, total, err := repo.List(models.AccountTypePersonal, 10, 0)
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

func TestInMemoryRepository_List_Pagination(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create 10 accounts
	for i := 0; i < 10; i++ {
		account := &models.Account{
			ID:    string(rune('a' + i)),
			Email: string(rune('a'+i)) + "@example.com",
		}
		repo.Create(account)
	}

	// First page
	accounts, total, _ := repo.List("", 3, 0)
	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts on first page, got %d", len(accounts))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Offset beyond count
	accounts, total, _ = repo.List("", 3, 100)
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts for offset beyond count, got %d", len(accounts))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
}

func TestInMemoryRepository_UpdateBalanceAtomic(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:      "acc_123",
		Email:   "test@example.com",
		Balance: 10000,
	}
	repo.Create(account)

	err := repo.UpdateBalanceAtomic("acc_123", 5000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, _ := repo.GetByID("acc_123")
	if result.Balance != 15000 {
		t.Errorf("expected balance 15000, got %d", result.Balance)
	}
}

func TestInMemoryRepository_UpdateBalanceAtomic_Debit(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:      "acc_123",
		Email:   "test@example.com",
		Balance: 10000,
	}
	repo.Create(account)

	err := repo.UpdateBalanceAtomic("acc_123", -3000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, _ := repo.GetByID("acc_123")
	if result.Balance != 7000 {
		t.Errorf("expected balance 7000, got %d", result.Balance)
	}
}

func TestInMemoryRepository_UpdateBalanceAtomic_InsufficientBalance(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:      "acc_123",
		Email:   "test@example.com",
		Balance: 5000,
	}
	repo.Create(account)

	err := repo.UpdateBalanceAtomic("acc_123", -10000)
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}

	// Balance should remain unchanged
	result, _ := repo.GetByID("acc_123")
	if result.Balance != 5000 {
		t.Errorf("expected balance 5000 unchanged, got %d", result.Balance)
	}
}

func TestInMemoryRepository_UpdateBalanceAtomic_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	err := repo.UpdateBalanceAtomic("nonexistent", 1000)
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_UpdateWithVersion(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "test@example.com",
	}
	repo.Create(account)

	// Version should be 1 after creation
	version, _ := repo.GetVersion("acc_123")
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Update with correct version
	account.Status = models.StatusActive
	err := repo.UpdateWithVersion(account, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Version should be 2 now
	version, _ = repo.GetVersion("acc_123")
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}
}

func TestInMemoryRepository_UpdateWithVersion_Conflict(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:    "acc_123",
		Email: "test@example.com",
	}
	repo.Create(account)

	// Try to update with wrong version
	err := repo.UpdateWithVersion(account, 999)
	if err != ErrConcurrentUpdate {
		t.Errorf("expected ErrConcurrentUpdate, got %v", err)
	}
}

func TestInMemoryRepository_GetVersion_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	_, err := repo.GetVersion("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestInMemoryRepository_ConcurrentAccess(t *testing.T) {
	repo := NewInMemoryRepository()

	account := &models.Account{
		ID:      "acc_123",
		Email:   "test@example.com",
		Balance: 100000,
	}
	repo.Create(account)

	// Run concurrent balance updates
	var wg sync.WaitGroup
	numGoroutines := 100
	creditAmount := int64(100)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo.UpdateBalanceAtomic("acc_123", creditAmount)
		}()
	}

	wg.Wait()

	result, _ := repo.GetByID("acc_123")
	expectedBalance := int64(100000 + numGoroutines*int(creditAmount))
	if result.Balance != expectedBalance {
		t.Errorf("expected balance %d, got %d", expectedBalance, result.Balance)
	}
}

func TestInMemoryRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create initial account
	account := &models.Account{
		ID:      "acc_123",
		Email:   "test@example.com",
		Balance: 10000,
	}
	repo.Create(account)

	var wg sync.WaitGroup
	numReaders := 50
	numWriters := 50

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = repo.GetByID("acc_123")
			}
		}()
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = repo.UpdateBalanceAtomic("acc_123", 1)
			}
		}()
	}

	wg.Wait()

	// Should complete without deadlock or panic
	result, err := repo.GetByID("acc_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Balance < 10000 {
		t.Errorf("balance should have increased, got %d", result.Balance)
	}
}
