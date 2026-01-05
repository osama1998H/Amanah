package repositories

import (
	"errors"
	"sync"
	"time"

	"amanah/services/transaction/models"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrDuplicateKey        = errors.New("duplicate idempotency key")
	ErrInvalidTransition   = errors.New("invalid status transition")
)

// TransactionRepository defines the interface for transaction storage
type TransactionRepository interface {
	Create(tx *models.Transaction) error
	GetByID(id string) (*models.Transaction, error)
	GetByIdempotencyKey(key string) (*models.Transaction, error)
	Update(tx *models.Transaction) error
	List(merchantID string, limit, offset int) ([]*models.Transaction, int, error)
}

// InMemoryRepository implements TransactionRepository with in-memory storage
type InMemoryRepository struct {
	mu              sync.RWMutex
	transactions    map[string]*models.Transaction
	idempotencyKeys map[string]string // key -> transaction ID
}

// NewInMemoryRepository creates a new in-memory repository
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		transactions:    make(map[string]*models.Transaction),
		idempotencyKeys: make(map[string]string),
	}
}

// Create stores a new transaction
func (r *InMemoryRepository) Create(tx *models.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate idempotency key
	if tx.IdempotencyKey != "" {
		if _, exists := r.idempotencyKeys[tx.IdempotencyKey]; exists {
			return ErrDuplicateKey
		}
		r.idempotencyKeys[tx.IdempotencyKey] = tx.ID
	}

	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt
	r.transactions[tx.ID] = tx
	return nil
}

// GetByID retrieves a transaction by ID
func (r *InMemoryRepository) GetByID(id string) (*models.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tx, exists := r.transactions[id]
	if !exists {
		return nil, ErrTransactionNotFound
	}
	return tx, nil
}

// GetByIdempotencyKey retrieves a transaction by idempotency key
func (r *InMemoryRepository) GetByIdempotencyKey(key string) (*models.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	txID, exists := r.idempotencyKeys[key]
	if !exists {
		return nil, ErrTransactionNotFound
	}
	return r.transactions[txID], nil
}

// Update modifies an existing transaction
func (r *InMemoryRepository) Update(tx *models.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transactions[tx.ID]; !exists {
		return ErrTransactionNotFound
	}

	tx.UpdatedAt = time.Now().UTC()
	r.transactions[tx.ID] = tx
	return nil
}

// List returns transactions for a merchant with pagination
func (r *InMemoryRepository) List(merchantID string, limit, offset int) ([]*models.Transaction, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*models.Transaction
	for _, tx := range r.transactions {
		if merchantID == "" || tx.MerchantID == merchantID {
			filtered = append(filtered, tx)
		}
	}

	total := len(filtered)

	// Apply pagination
	if offset >= len(filtered) {
		return []*models.Transaction{}, total, nil
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], total, nil
}
