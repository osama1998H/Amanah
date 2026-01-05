package repositories

import (
	"errors"
	"sync"
	"time"

	"amanah/services/account/models"
)

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInstrumentNotFound = errors.New("payment instrument not found")
	ErrConcurrentUpdate   = errors.New("concurrent update detected")
)

// AccountRepository defines the interface for account storage
type AccountRepository interface {
	Create(account *models.Account) error
	GetByID(id string) (*models.Account, error)
	GetByEmail(email string) (*models.Account, error)
	Update(account *models.Account) error
	UpdateWithVersion(account *models.Account, expectedVersion int64) error
	Delete(id string) error
	List(accountType models.AccountType, limit, offset int) ([]*models.Account, int, error)
}

// InMemoryRepository implements AccountRepository with in-memory storage
type InMemoryRepository struct {
	mu           sync.RWMutex
	accounts     map[string]*models.Account
	emailIndex   map[string]string // email -> account ID
	versions     map[string]int64  // account ID -> version for optimistic locking
}

// NewInMemoryRepository creates a new in-memory repository
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		accounts:   make(map[string]*models.Account),
		emailIndex: make(map[string]string),
		versions:   make(map[string]int64),
	}
}

// Create stores a new account
func (r *InMemoryRepository) Create(account *models.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate email
	if _, exists := r.emailIndex[account.Email]; exists {
		return ErrEmailAlreadyExists
	}

	account.CreatedAt = time.Now().UTC()
	account.UpdatedAt = account.CreatedAt
	r.accounts[account.ID] = account
	r.emailIndex[account.Email] = account.ID
	r.versions[account.ID] = 1

	return nil
}

// GetByID retrieves an account by ID
func (r *InMemoryRepository) GetByID(id string) (*models.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	account, exists := r.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

// GetByEmail retrieves an account by email
func (r *InMemoryRepository) GetByEmail(email string) (*models.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.emailIndex[email]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return r.accounts[id], nil
}

// Update modifies an existing account
func (r *InMemoryRepository) Update(account *models.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.accounts[account.ID]
	if !exists {
		return ErrAccountNotFound
	}

	// Update email index if email changed
	if existing.Email != account.Email {
		delete(r.emailIndex, existing.Email)
		r.emailIndex[account.Email] = account.ID
	}

	account.UpdatedAt = time.Now().UTC()
	r.accounts[account.ID] = account
	r.versions[account.ID]++

	return nil
}

// UpdateWithVersion performs optimistic locking update
func (r *InMemoryRepository) UpdateWithVersion(account *models.Account, expectedVersion int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.accounts[account.ID]; !exists {
		return ErrAccountNotFound
	}

	if r.versions[account.ID] != expectedVersion {
		return ErrConcurrentUpdate
	}

	account.UpdatedAt = time.Now().UTC()
	r.accounts[account.ID] = account
	r.versions[account.ID]++

	return nil
}

// Delete removes an account
func (r *InMemoryRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	account, exists := r.accounts[id]
	if !exists {
		return ErrAccountNotFound
	}

	delete(r.emailIndex, account.Email)
	delete(r.accounts, id)
	delete(r.versions, id)

	return nil
}

// List returns accounts with optional filtering and pagination
func (r *InMemoryRepository) List(accountType models.AccountType, limit, offset int) ([]*models.Account, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*models.Account
	for _, account := range r.accounts {
		if accountType == "" || account.Type == accountType {
			filtered = append(filtered, account)
		}
	}

	total := len(filtered)

	// Apply pagination
	if offset >= len(filtered) {
		return []*models.Account{}, total, nil
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], total, nil
}

// GetVersion returns the current version of an account
func (r *InMemoryRepository) GetVersion(id string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	version, exists := r.versions[id]
	if !exists {
		return 0, ErrAccountNotFound
	}
	return version, nil
}
