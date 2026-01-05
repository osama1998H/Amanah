package testing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FixtureManager manages test fixtures
type FixtureManager struct {
	mu          sync.Mutex
	fixtures    map[string]interface{}
	fixturesDir string
	loaded      bool
}

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(dir string) *FixtureManager {
	return &FixtureManager{
		fixtures:    make(map[string]interface{}),
		fixturesDir: dir,
	}
}

// Load loads fixtures from JSON files in the fixtures directory
func (fm *FixtureManager) Load() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.loaded {
		return nil
	}

	entries, err := os.ReadDir(fm.fixturesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fm.loaded = true
			return nil // No fixtures directory is OK
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(fm.fixturesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read fixture %s: %w", entry.Name(), err)
		}

		var fixture interface{}
		if err := json.Unmarshal(data, &fixture); err != nil {
			return fmt.Errorf("failed to parse fixture %s: %w", entry.Name(), err)
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		fm.fixtures[name] = fixture
	}

	fm.loaded = true
	return nil
}

// Get retrieves a fixture by name
func (fm *FixtureManager) Get(name string) (interface{}, bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	v, ok := fm.fixtures[name]
	return v, ok
}

// GetAs retrieves a fixture and unmarshals it into the target
func (fm *FixtureManager) GetAs(name string, target interface{}) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	v, ok := fm.fixtures[name]
	if !ok {
		return fmt.Errorf("fixture not found: %s", name)
	}

	// Re-marshal and unmarshal to target type
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

// Set stores a fixture
func (fm *FixtureManager) Set(name string, value interface{}) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.fixtures[name] = value
}

// Clear removes all fixtures
func (fm *FixtureManager) Clear() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.fixtures = make(map[string]interface{})
	fm.loaded = false
}

// TestUser represents a test user fixture
type TestUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // Hashed
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TestAccount represents a test account fixture
type TestAccount struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Currency  string    `json:"currency"`
	Balance   float64   `json:"balance"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TestTransaction represents a test transaction fixture
type TestTransaction struct {
	ID              string    `json:"id"`
	FromAccountID   string    `json:"from_account_id"`
	ToAccountID     string    `json:"to_account_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	Reference       string    `json:"reference"`
	IdempotencyKey  string    `json:"idempotency_key"`
	CreatedAt       time.Time `json:"created_at"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
}

// TestMerchant represents a test merchant fixture
type TestMerchant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	APIKey        string    `json:"api_key"`
	WebhookURL    string    `json:"webhook_url"`
	Status        string    `json:"status"`
	FeePercentage float64   `json:"fee_percentage"`
	CreatedAt     time.Time `json:"created_at"`
}

// FixtureFactory creates test fixtures with realistic data
type FixtureFactory struct {
	counter int
	mu      sync.Mutex
}

// NewFixtureFactory creates a new fixture factory
func NewFixtureFactory() *FixtureFactory {
	return &FixtureFactory{}
}

// nextID generates a unique ID
func (f *FixtureFactory) nextID(prefix string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counter++
	return fmt.Sprintf("%s_%d_%s", prefix, f.counter, randomHex(4))
}

// randomHex generates random hex string
func randomHex(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CreateUser creates a test user
func (f *FixtureFactory) CreateUser(opts ...func(*TestUser)) TestUser {
	user := TestUser{
		ID:        f.nextID("usr"),
		Email:     fmt.Sprintf("user%d@example.com", f.counter),
		Username:  fmt.Sprintf("testuser%d", f.counter),
		Password:  "$2a$10$example_hashed_password",
		Role:      "user",
		Status:    "active",
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(&user)
	}

	return user
}

// CreateAccount creates a test account
func (f *FixtureFactory) CreateAccount(userID string, opts ...func(*TestAccount)) TestAccount {
	account := TestAccount{
		ID:        f.nextID("acc"),
		UserID:    userID,
		Type:      "savings",
		Currency:  "USD",
		Balance:   1000.00,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(&account)
	}

	return account
}

// CreateTransaction creates a test transaction
func (f *FixtureFactory) CreateTransaction(fromID, toID string, opts ...func(*TestTransaction)) TestTransaction {
	tx := TestTransaction{
		ID:             f.nextID("txn"),
		FromAccountID:  fromID,
		ToAccountID:    toID,
		Amount:         100.00,
		Currency:       "USD",
		Type:           "transfer",
		Status:         "pending",
		Reference:      fmt.Sprintf("REF%d", f.counter),
		IdempotencyKey: randomHex(16),
		CreatedAt:      time.Now(),
	}

	for _, opt := range opts {
		opt(&tx)
	}

	return tx
}

// CreateMerchant creates a test merchant
func (f *FixtureFactory) CreateMerchant(opts ...func(*TestMerchant)) TestMerchant {
	merchant := TestMerchant{
		ID:            f.nextID("mch"),
		Name:          fmt.Sprintf("Test Merchant %d", f.counter),
		APIKey:        fmt.Sprintf("sk_test_%s", randomHex(24)),
		WebhookURL:    fmt.Sprintf("https://merchant%d.example.com/webhook", f.counter),
		Status:        "active",
		FeePercentage: 2.5,
		CreatedAt:     time.Now(),
	}

	for _, opt := range opts {
		opt(&merchant)
	}

	return merchant
}

// User option functions

// WithUserEmail sets the user email
func WithUserEmail(email string) func(*TestUser) {
	return func(u *TestUser) {
		u.Email = email
	}
}

// WithUserRole sets the user role
func WithUserRole(role string) func(*TestUser) {
	return func(u *TestUser) {
		u.Role = role
	}
}

// WithUserStatus sets the user status
func WithUserStatus(status string) func(*TestUser) {
	return func(u *TestUser) {
		u.Status = status
	}
}

// Account option functions

// WithAccountBalance sets the account balance
func WithAccountBalance(balance float64) func(*TestAccount) {
	return func(a *TestAccount) {
		a.Balance = balance
	}
}

// WithAccountCurrency sets the account currency
func WithAccountCurrency(currency string) func(*TestAccount) {
	return func(a *TestAccount) {
		a.Currency = currency
	}
}

// WithAccountType sets the account type
func WithAccountType(accountType string) func(*TestAccount) {
	return func(a *TestAccount) {
		a.Type = accountType
	}
}

// Transaction option functions

// WithTransactionAmount sets the transaction amount
func WithTransactionAmount(amount float64) func(*TestTransaction) {
	return func(t *TestTransaction) {
		t.Amount = amount
	}
}

// WithTransactionStatus sets the transaction status
func WithTransactionStatus(status string) func(*TestTransaction) {
	return func(t *TestTransaction) {
		t.Status = status
	}
}

// WithTransactionType sets the transaction type
func WithTransactionType(txType string) func(*TestTransaction) {
	return func(t *TestTransaction) {
		t.Type = txType
	}
}

// Merchant option functions

// WithMerchantFee sets the merchant fee percentage
func WithMerchantFee(fee float64) func(*TestMerchant) {
	return func(m *TestMerchant) {
		m.FeePercentage = fee
	}
}

// WithMerchantStatus sets the merchant status
func WithMerchantStatus(status string) func(*TestMerchant) {
	return func(m *TestMerchant) {
		m.Status = status
	}
}

// TestData holds common test data sets
type TestData struct {
	Users        []TestUser
	Accounts     []TestAccount
	Transactions []TestTransaction
	Merchants    []TestMerchant
}

// CreateTestData creates a complete set of test data
func (f *FixtureFactory) CreateTestData() TestData {
	data := TestData{}

	// Create users
	admin := f.CreateUser(WithUserRole("admin"), WithUserEmail("admin@example.com"))
	user1 := f.CreateUser(WithUserEmail("user1@example.com"))
	user2 := f.CreateUser(WithUserEmail("user2@example.com"))
	data.Users = []TestUser{admin, user1, user2}

	// Create accounts
	acc1 := f.CreateAccount(user1.ID, WithAccountBalance(5000))
	acc2 := f.CreateAccount(user1.ID, WithAccountType("checking"), WithAccountBalance(1000))
	acc3 := f.CreateAccount(user2.ID, WithAccountBalance(3000))
	data.Accounts = []TestAccount{acc1, acc2, acc3}

	// Create transactions
	tx1 := f.CreateTransaction(acc1.ID, acc3.ID, WithTransactionAmount(100), WithTransactionStatus("completed"))
	tx2 := f.CreateTransaction(acc2.ID, acc3.ID, WithTransactionAmount(50), WithTransactionStatus("pending"))
	data.Transactions = []TestTransaction{tx1, tx2}

	// Create merchants
	merchant := f.CreateMerchant()
	data.Merchants = []TestMerchant{merchant}

	return data
}

// EnvironmentFixtures sets up test environment variables
type EnvironmentFixtures struct {
	original map[string]string
	set      map[string]string
}

// NewEnvironmentFixtures creates a new environment fixture handler
func NewEnvironmentFixtures() *EnvironmentFixtures {
	return &EnvironmentFixtures{
		original: make(map[string]string),
		set:      make(map[string]string),
	}
}

// Set sets an environment variable and saves the original value
func (e *EnvironmentFixtures) Set(key, value string) {
	if _, saved := e.original[key]; !saved {
		e.original[key] = os.Getenv(key)
	}
	os.Setenv(key, value)
	e.set[key] = value
}

// Restore restores all original environment values
func (e *EnvironmentFixtures) Restore() {
	for key, value := range e.original {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
	e.original = make(map[string]string)
	e.set = make(map[string]string)
}

// SetTestEnvironment sets common test environment variables
func (e *EnvironmentFixtures) SetTestEnvironment() {
	e.Set("ENV", "test")
	e.Set("LOG_LEVEL", "debug")
	e.Set("DB_HOST", "localhost")
	e.Set("DB_PORT", "5432")
	e.Set("DB_NAME", "amanah_test")
	e.Set("DB_USER", "test_user")
	e.Set("DB_PASSWORD", "test_password")
	e.Set("REDIS_HOST", "localhost")
	e.Set("REDIS_PORT", "6379")
	e.Set("JWT_SECRET", "test-jwt-secret-key-32-chars-min")
	e.Set("API_KEY_SECRET", "test-api-key-secret")
}
