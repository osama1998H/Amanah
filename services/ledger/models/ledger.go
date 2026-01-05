package models

import (
	"time"
)

// EntryType represents the type of ledger entry
type EntryType string

const (
	EntryDebit  EntryType = "debit"
	EntryCredit EntryType = "credit"
)

// AccountType represents the type of ledger account
type AccountType string

const (
	AccountAsset     AccountType = "asset"
	AccountLiability AccountType = "liability"
	AccountEquity    AccountType = "equity"
	AccountRevenue   AccountType = "revenue"
	AccountExpense   AccountType = "expense"
)

// LedgerEntry represents an immutable ledger entry
type LedgerEntry struct {
	ID            string            `json:"id"`
	JournalID     string            `json:"journal_id"`     // Groups related entries
	AccountID     string            `json:"account_id"`
	Type          EntryType         `json:"type"`
	Amount        int64             `json:"amount"`         // Amount in cents
	Currency      string            `json:"currency"`
	Balance       int64             `json:"balance"`        // Running balance after entry
	Description   string            `json:"description"`
	ReferenceType string            `json:"reference_type"` // transaction, refund, adjustment
	ReferenceID   string            `json:"reference_id"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

// JournalEntry represents a double-entry journal with debits and credits
type JournalEntry struct {
	ID            string            `json:"id"`
	Description   string            `json:"description"`
	ReferenceType string            `json:"reference_type"`
	ReferenceID   string            `json:"reference_id"`
	Entries       []LedgerEntry     `json:"entries"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

// LedgerAccount represents an account in the ledger
type LedgerAccount struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Type         AccountType `json:"type"`
	Currency     string      `json:"currency"`
	Balance      int64       `json:"balance"`
	NormalBalance EntryType  `json:"normal_balance"` // debit or credit
	IsActive     bool        `json:"is_active"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          string            `json:"id"`
	Action      string            `json:"action"`
	EntityType  string            `json:"entity_type"`
	EntityID    string            `json:"entity_id"`
	ActorType   string            `json:"actor_type"`   // user, system, api
	ActorID     string            `json:"actor_id"`
	OldValue    string            `json:"old_value,omitempty"`
	NewValue    string            `json:"new_value,omitempty"`
	IPAddress   string            `json:"ip_address,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// CreateJournalRequest represents a request to create a journal entry
type CreateJournalRequest struct {
	Description   string            `json:"description"`
	ReferenceType string            `json:"reference_type"`
	ReferenceID   string            `json:"reference_id"`
	Entries       []EntryRequest    `json:"entries"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// EntryRequest represents a single entry in a journal request
type EntryRequest struct {
	AccountID   string    `json:"account_id"`
	Type        EntryType `json:"type"`
	Amount      int64     `json:"amount"`
	Description string    `json:"description,omitempty"`
}

// LedgerResponse wraps ledger data for API responses
type LedgerResponse struct {
	Success bool           `json:"success"`
	Journal *JournalEntry  `json:"journal,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// AccountResponse wraps account data for API responses
type AccountResponse struct {
	Success bool           `json:"success"`
	Account *LedgerAccount `json:"account,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// EntriesResponse wraps ledger entries for API responses
type EntriesResponse struct {
	Success bool           `json:"success"`
	Entries []LedgerEntry  `json:"entries"`
	Total   int            `json:"total"`
}

// AuditLogsResponse wraps audit logs for API responses
type AuditLogsResponse struct {
	Success bool        `json:"success"`
	Logs    []AuditLog  `json:"logs"`
	Total   int         `json:"total"`
}
