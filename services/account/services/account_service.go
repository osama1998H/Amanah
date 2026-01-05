package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"amanah/services/account/models"
	"amanah/services/account/repositories"
)

var (
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrInvalidType      = errors.New("invalid account type")
	ErrInvalidCurrency  = errors.New("invalid currency")
	ErrAccountFrozen    = errors.New("account is frozen")
	ErrAccountClosed    = errors.New("account is closed")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrKYCRequired      = errors.New("KYC verification required")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// SupportedCurrencies lists valid currency codes
var SupportedCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "IQD": true, "AED": true,
}

// AccountService handles account business logic
type AccountService struct {
	repo repositories.AccountRepository
}

// NewAccountService creates a new account service
func NewAccountService(repo repositories.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// CreateAccount creates a new account
func (s *AccountService) CreateAccount(req *models.CreateAccountRequest) (*models.Account, error) {
	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if email already exists
	if _, err := s.repo.GetByEmail(req.Email); err == nil {
		return nil, repositories.ErrEmailAlreadyExists
	}

	// Generate account ID
	accountID, err := generateID("acc")
	if err != nil {
		return nil, err
	}

	account := &models.Account{
		ID:        accountID,
		Email:     req.Email,
		Type:      req.Type,
		Status:    models.StatusPending,
		KYCStatus: models.KYCNotStarted,
		Profile:   req.Profile,
		Currency:  req.Currency,
		Metadata:  req.Metadata,
		Balance:   0,
	}

	if err := s.repo.Create(account); err != nil {
		return nil, err
	}

	return account, nil
}

// GetAccount retrieves an account by ID
func (s *AccountService) GetAccount(id string) (*models.Account, error) {
	return s.repo.GetByID(id)
}

// GetAccountByEmail retrieves an account by email
func (s *AccountService) GetAccountByEmail(email string) (*models.Account, error) {
	return s.repo.GetByEmail(email)
}

// UpdateAccount updates account profile information
func (s *AccountService) UpdateAccount(id string, req *models.UpdateAccountRequest) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccountActive(account); err != nil {
		return nil, err
	}

	if req.Profile != nil {
		account.Profile = req.Profile
	}
	if req.Metadata != nil {
		if account.Metadata == nil {
			account.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			account.Metadata[k] = v
		}
	}

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// ActivateAccount activates a pending account
func (s *AccountService) ActivateAccount(id string) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if account.Status == models.StatusClosed {
		return nil, ErrAccountClosed
	}

	account.Status = models.StatusActive

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// FreezeAccount freezes an account
func (s *AccountService) FreezeAccount(id string, reason string) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	account.Status = models.StatusFrozen
	if account.Metadata == nil {
		account.Metadata = make(map[string]string)
	}
	account.Metadata["freeze_reason"] = reason

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// UnfreezeAccount unfreezes an account
func (s *AccountService) UnfreezeAccount(id string) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if account.Status != models.StatusFrozen {
		return account, nil
	}

	account.Status = models.StatusActive
	delete(account.Metadata, "freeze_reason")

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// CloseAccount permanently closes an account
func (s *AccountService) CloseAccount(id string) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	account.Status = models.StatusClosed

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// UpdateKYCStatus updates the KYC verification status
func (s *AccountService) UpdateKYCStatus(id string, status models.KYCStatus) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	account.KYCStatus = status
	if status == models.KYCVerified {
		now := time.Now().UTC()
		account.VerifiedAt = &now
	}

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// AddPaymentInstrument adds a payment method to an account
func (s *AccountService) AddPaymentInstrument(accountID string, req *models.AddPaymentInstrumentRequest) (*models.Account, error) {
	account, err := s.repo.GetByID(accountID)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccountActive(account); err != nil {
		return nil, err
	}

	instrID, _ := generateID("pi")
	instrument := models.PaymentInstrument{
		ID:        instrID,
		Type:      req.Type,
		IsDefault: req.IsDefault,
		CreatedAt: time.Now().UTC(),
	}

	// If this is default, unset others
	if req.IsDefault {
		for i := range account.PaymentInstruments {
			account.PaymentInstruments[i].IsDefault = false
		}
	}

	account.PaymentInstruments = append(account.PaymentInstruments, instrument)

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// RemovePaymentInstrument removes a payment method from an account
func (s *AccountService) RemovePaymentInstrument(accountID, instrumentID string) (*models.Account, error) {
	account, err := s.repo.GetByID(accountID)
	if err != nil {
		return nil, err
	}

	found := false
	newInstruments := make([]models.PaymentInstrument, 0)
	for _, inst := range account.PaymentInstruments {
		if inst.ID != instrumentID {
			newInstruments = append(newInstruments, inst)
		} else {
			found = true
		}
	}

	if !found {
		return nil, repositories.ErrInstrumentNotFound
	}

	account.PaymentInstruments = newInstruments

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// CreditBalance adds funds to an account
func (s *AccountService) CreditBalance(id string, amount int64) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccountActive(account); err != nil {
		return nil, err
	}

	account.Balance += amount

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// DebitBalance removes funds from an account
func (s *AccountService) DebitBalance(id string, amount int64) (*models.Account, error) {
	account, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccountActive(account); err != nil {
		return nil, err
	}

	if account.Balance < amount {
		return nil, ErrInsufficientBalance
	}

	account.Balance -= amount

	if err := s.repo.Update(account); err != nil {
		return nil, err
	}

	return account, nil
}

// ListAccounts returns accounts with filtering and pagination
func (s *AccountService) ListAccounts(accountType models.AccountType, limit, offset int) ([]*models.Account, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(accountType, limit, offset)
}

// validateCreateRequest validates a create account request
func (s *AccountService) validateCreateRequest(req *models.CreateAccountRequest) error {
	if !emailRegex.MatchString(req.Email) {
		return ErrInvalidEmail
	}

	validTypes := map[models.AccountType]bool{
		models.AccountTypePersonal: true,
		models.AccountTypeBusiness: true,
		models.AccountTypeMerchant: true,
	}
	if !validTypes[req.Type] {
		return ErrInvalidType
	}

	if !SupportedCurrencies[req.Currency] {
		return ErrInvalidCurrency
	}

	return nil
}

// checkAccountActive verifies the account can perform operations
func (s *AccountService) checkAccountActive(account *models.Account) error {
	switch account.Status {
	case models.StatusFrozen:
		return ErrAccountFrozen
	case models.StatusClosed:
		return ErrAccountClosed
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
