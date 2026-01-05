package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"amanah/services/ledger/models"
)

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrUnbalancedEntry    = errors.New("journal entry debits and credits must balance")
	ErrEmptyJournal       = errors.New("journal must have at least two entries")
	ErrInvalidAmount      = errors.New("amount must be positive")
	ErrAccountInactive    = errors.New("account is inactive")
)

// LedgerService handles double-entry accounting
type LedgerService struct {
	mu sync.RWMutex

	accounts   map[string]*models.LedgerAccount
	entries    []models.LedgerEntry
	journals   map[string]*models.JournalEntry
	auditLogs  []models.AuditLog
}

// NewLedgerService creates a new ledger service
func NewLedgerService() *LedgerService {
	svc := &LedgerService{
		accounts:  make(map[string]*models.LedgerAccount),
		entries:   make([]models.LedgerEntry, 0),
		journals:  make(map[string]*models.JournalEntry),
		auditLogs: make([]models.AuditLog, 0),
	}

	// Initialize default system accounts
	svc.initializeSystemAccounts()

	return svc
}

// initializeSystemAccounts creates standard ledger accounts
func (s *LedgerService) initializeSystemAccounts() {
	defaultAccounts := []struct {
		id       string
		name     string
		typ      models.AccountType
		normal   models.EntryType
	}{
		{"acc_cash", "Cash", models.AccountAsset, models.EntryDebit},
		{"acc_receivables", "Accounts Receivable", models.AccountAsset, models.EntryDebit},
		{"acc_payables", "Accounts Payable", models.AccountLiability, models.EntryCredit},
		{"acc_revenue", "Revenue", models.AccountRevenue, models.EntryCredit},
		{"acc_fees", "Transaction Fees", models.AccountRevenue, models.EntryCredit},
		{"acc_refunds", "Refunds", models.AccountExpense, models.EntryDebit},
		{"acc_merchant_payable", "Merchant Payable", models.AccountLiability, models.EntryCredit},
		{"acc_customer_deposits", "Customer Deposits", models.AccountLiability, models.EntryCredit},
	}

	for _, acc := range defaultAccounts {
		s.accounts[acc.id] = &models.LedgerAccount{
			ID:            acc.id,
			Name:          acc.name,
			Type:          acc.typ,
			Currency:      "USD",
			Balance:       0,
			NormalBalance: acc.normal,
			IsActive:      true,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
	}
}

// CreateAccount creates a new ledger account
func (s *LedgerService) CreateAccount(name string, accountType models.AccountType, currency string) (*models.LedgerAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, _ := generateID("acc")

	// Determine normal balance based on account type
	normalBalance := models.EntryCredit
	if accountType == models.AccountAsset || accountType == models.AccountExpense {
		normalBalance = models.EntryDebit
	}

	account := &models.LedgerAccount{
		ID:            id,
		Name:          name,
		Type:          accountType,
		Currency:      currency,
		Balance:       0,
		NormalBalance: normalBalance,
		IsActive:      true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	s.accounts[account.ID] = account

	s.logAudit("create", "account", account.ID, "system", "system", "", account.ID, nil)

	return account, nil
}

// GetAccount retrieves an account by ID
func (s *LedgerService) GetAccount(id string) (*models.LedgerAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

// ListAccounts returns all accounts
func (s *LedgerService) ListAccounts() []*models.LedgerAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]*models.LedgerAccount, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

// CreateJournalEntry creates a balanced double-entry journal
func (s *LedgerService) CreateJournalEntry(req *models.CreateJournalRequest) (*models.JournalEntry, error) {
	if len(req.Entries) < 2 {
		return nil, ErrEmptyJournal
	}

	// Validate entries balance
	if err := s.validateBalance(req.Entries); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all accounts exist and are active
	for _, entry := range req.Entries {
		account, exists := s.accounts[entry.AccountID]
		if !exists {
			return nil, ErrAccountNotFound
		}
		if !account.IsActive {
			return nil, ErrAccountInactive
		}
		if entry.Amount <= 0 {
			return nil, ErrInvalidAmount
		}
	}

	journalID, _ := generateID("jnl")
	now := time.Now().UTC()

	journal := &models.JournalEntry{
		ID:            journalID,
		Description:   req.Description,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		Entries:       make([]models.LedgerEntry, 0, len(req.Entries)),
		Metadata:      req.Metadata,
		CreatedAt:     now,
	}

	// Create ledger entries and update balances
	for _, entryReq := range req.Entries {
		account := s.accounts[entryReq.AccountID]

		// Update account balance
		if entryReq.Type == models.EntryDebit {
			if account.NormalBalance == models.EntryDebit {
				account.Balance += entryReq.Amount
			} else {
				account.Balance -= entryReq.Amount
			}
		} else {
			if account.NormalBalance == models.EntryCredit {
				account.Balance += entryReq.Amount
			} else {
				account.Balance -= entryReq.Amount
			}
		}
		account.UpdatedAt = now

		entryID, _ := generateID("ent")
		entry := models.LedgerEntry{
			ID:            entryID,
			JournalID:     journalID,
			AccountID:     entryReq.AccountID,
			Type:          entryReq.Type,
			Amount:        entryReq.Amount,
			Currency:      account.Currency,
			Balance:       account.Balance,
			Description:   entryReq.Description,
			ReferenceType: req.ReferenceType,
			ReferenceID:   req.ReferenceID,
			Metadata:      req.Metadata,
			CreatedAt:     now,
		}

		journal.Entries = append(journal.Entries, entry)
		s.entries = append(s.entries, entry)
	}

	s.journals[journal.ID] = journal

	s.logAudit("create", "journal", journal.ID, "system", "system", "", journal.ID, req.Metadata)

	return journal, nil
}

// validateBalance ensures debits equal credits
func (s *LedgerService) validateBalance(entries []models.EntryRequest) error {
	var totalDebits, totalCredits int64

	for _, entry := range entries {
		if entry.Type == models.EntryDebit {
			totalDebits += entry.Amount
		} else {
			totalCredits += entry.Amount
		}
	}

	if totalDebits != totalCredits {
		return ErrUnbalancedEntry
	}

	return nil
}

// GetJournal retrieves a journal entry by ID
func (s *LedgerService) GetJournal(id string) (*models.JournalEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	journal, exists := s.journals[id]
	if !exists {
		return nil, errors.New("journal not found")
	}
	return journal, nil
}

// GetEntriesForAccount retrieves all entries for an account
func (s *LedgerService) GetEntriesForAccount(accountID string, limit, offset int) ([]models.LedgerEntry, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []models.LedgerEntry
	for _, entry := range s.entries {
		if entry.AccountID == accountID {
			filtered = append(filtered, entry)
		}
	}

	total := len(filtered)
	if offset >= total {
		return []models.LedgerEntry{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total
}

// GetEntriesForReference retrieves entries by reference
func (s *LedgerService) GetEntriesForReference(refType, refID string) []models.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []models.LedgerEntry
	for _, entry := range s.entries {
		if entry.ReferenceType == refType && entry.ReferenceID == refID {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// RecordPayment records a payment transaction in the ledger
func (s *LedgerService) RecordPayment(transactionID string, amount int64, merchantAccountID string) (*models.JournalEntry, error) {
	req := &models.CreateJournalRequest{
		Description:   "Payment received",
		ReferenceType: "transaction",
		ReferenceID:   transactionID,
		Entries: []models.EntryRequest{
			{AccountID: "acc_cash", Type: models.EntryDebit, Amount: amount},
			{AccountID: "acc_customer_deposits", Type: models.EntryCredit, Amount: amount},
		},
	}

	return s.CreateJournalEntry(req)
}

// RecordRefund records a refund in the ledger
func (s *LedgerService) RecordRefund(transactionID string, amount int64) (*models.JournalEntry, error) {
	req := &models.CreateJournalRequest{
		Description:   "Refund issued",
		ReferenceType: "refund",
		ReferenceID:   transactionID,
		Entries: []models.EntryRequest{
			{AccountID: "acc_refunds", Type: models.EntryDebit, Amount: amount},
			{AccountID: "acc_cash", Type: models.EntryCredit, Amount: amount},
		},
	}

	return s.CreateJournalEntry(req)
}

// RecordFee records a transaction fee
func (s *LedgerService) RecordFee(transactionID string, amount int64) (*models.JournalEntry, error) {
	req := &models.CreateJournalRequest{
		Description:   "Transaction fee",
		ReferenceType: "fee",
		ReferenceID:   transactionID,
		Entries: []models.EntryRequest{
			{AccountID: "acc_receivables", Type: models.EntryDebit, Amount: amount},
			{AccountID: "acc_fees", Type: models.EntryCredit, Amount: amount},
		},
	}

	return s.CreateJournalEntry(req)
}

// LogAudit records an audit log entry
func (s *LedgerService) LogAudit(action, entityType, entityID, actorType, actorID, oldValue, newValue string, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logAudit(action, entityType, entityID, actorType, actorID, oldValue, newValue, metadata)
}

// logAudit internal audit logging (must be called with lock held)
func (s *LedgerService) logAudit(action, entityType, entityID, actorType, actorID, oldValue, newValue string, metadata map[string]string) {
	id, _ := generateID("aud")
	log := models.AuditLog{
		ID:         id,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		ActorType:  actorType,
		ActorID:    actorID,
		OldValue:   oldValue,
		NewValue:   newValue,
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC(),
	}
	s.auditLogs = append(s.auditLogs, log)
}

// GetAuditLogs retrieves audit logs with pagination
func (s *LedgerService) GetAuditLogs(entityType, entityID string, limit, offset int) ([]models.AuditLog, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []models.AuditLog
	for _, log := range s.auditLogs {
		if (entityType == "" || log.EntityType == entityType) &&
			(entityID == "" || log.EntityID == entityID) {
			filtered = append(filtered, log)
		}
	}

	total := len(filtered)
	if offset >= total {
		return []models.AuditLog{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total
}

// GetTrialBalance returns the trial balance (sum of all account balances)
func (s *LedgerService) GetTrialBalance() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	balances := make(map[string]int64)
	for id, account := range s.accounts {
		balances[id] = account.Balance
	}
	return balances
}

// generateID creates a unique ID with a prefix
func generateID(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}
