package models

import (
	"time"
)

// AccountStatus represents the state of an account
type AccountStatus string

const (
	StatusActive    AccountStatus = "active"
	StatusPending   AccountStatus = "pending"
	StatusFrozen    AccountStatus = "frozen"
	StatusSuspended AccountStatus = "suspended"
	StatusClosed    AccountStatus = "closed"
)

// KYCStatus represents KYC verification status
type KYCStatus string

const (
	KYCNotStarted  KYCStatus = "not_started"
	KYCPending     KYCStatus = "pending"
	KYCVerified    KYCStatus = "verified"
	KYCRejected    KYCStatus = "rejected"
	KYCExpired     KYCStatus = "expired"
)

// AccountType represents the type of account
type AccountType string

const (
	AccountTypePersonal AccountType = "personal"
	AccountTypeBusiness AccountType = "business"
	AccountTypeMerchant AccountType = "merchant"
)

// Account represents a user or merchant account
type Account struct {
	ID                string            `json:"id"`
	Email             string            `json:"email"`
	Type              AccountType       `json:"type"`
	Status            AccountStatus     `json:"status"`
	KYCStatus         KYCStatus         `json:"kyc_status"`
	Profile           *Profile          `json:"profile,omitempty"`
	PaymentInstruments []PaymentInstrument `json:"payment_instruments,omitempty"`
	Balance           int64             `json:"balance"` // Balance in cents
	Currency          string            `json:"currency"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	VerifiedAt        *time.Time        `json:"verified_at,omitempty"`
}

// Profile contains account profile information
type Profile struct {
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Address     *Address `json:"address,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
	BusinessName string `json:"business_name,omitempty"`
	TaxID       string `json:"tax_id,omitempty"`
}

// Address represents a physical address
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// PaymentInstrument represents a linked payment method
type PaymentInstrument struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // card, bank_account, wallet
	Last4     string    `json:"last4,omitempty"`
	Brand     string    `json:"brand,omitempty"`
	ExpMonth  int       `json:"exp_month,omitempty"`
	ExpYear   int       `json:"exp_year,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAccountRequest represents a request to create an account
type CreateAccountRequest struct {
	Email    string            `json:"email"`
	Type     AccountType       `json:"type"`
	Currency string            `json:"currency"`
	Profile  *Profile          `json:"profile,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UpdateAccountRequest represents a request to update an account
type UpdateAccountRequest struct {
	Profile  *Profile          `json:"profile,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AddPaymentInstrumentRequest represents a request to add a payment method
type AddPaymentInstrumentRequest struct {
	Type      string `json:"type"`
	Token     string `json:"token"` // Tokenized payment info from provider
	IsDefault bool   `json:"is_default"`
}

// AccountResponse wraps an account for API responses
type AccountResponse struct {
	Success bool     `json:"success"`
	Account *Account `json:"account,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// AccountListResponse wraps a list of accounts
type AccountListResponse struct {
	Success  bool       `json:"success"`
	Accounts []*Account `json:"accounts"`
	Total    int        `json:"total"`
}
