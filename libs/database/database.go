package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrDuplicateKey   = errors.New("duplicate key")
	ErrConnectionFailed = errors.New("database connection failed")
	ErrTransactionFailed = errors.New("transaction failed")
)

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns default database configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "",
		Database:        "amanah",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// DB wraps sql.DB with additional functionality
type DB struct {
	*sql.DB
	config *Config
}

// New creates a new database connection
func New(config *Config) (*DB, error) {
	db, err := sql.Open("postgres", config.DSN())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	return &DB{DB: db, config: config}, nil
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w: %v (rollback: %v)", ErrTransactionFailed, err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}

	return nil
}

// Healthcheck verifies the database connection is alive
func (db *DB) Healthcheck(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// MockDB provides an in-memory mock for testing
type MockDB struct {
	mu      sync.RWMutex
	data    map[string]map[string]interface{}
	healthy bool
}

// NewMockDB creates a new mock database
func NewMockDB() *MockDB {
	return &MockDB{
		data:    make(map[string]map[string]interface{}),
		healthy: true,
	}
}

// Set stores a value
func (m *MockDB) Set(table, key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] == nil {
		m.data[table] = make(map[string]interface{})
	}
	m.data[table][key] = value
}

// Get retrieves a value
func (m *MockDB) Get(table, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.data[table] == nil {
		return nil, false
	}
	val, ok := m.data[table][key]
	return val, ok
}

// Delete removes a value
func (m *MockDB) Delete(table, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] != nil {
		delete(m.data[table], key)
	}
}

// Healthcheck returns mock health status
func (m *MockDB) Healthcheck(ctx context.Context) error {
	if !m.healthy {
		return ErrConnectionFailed
	}
	return nil
}

// SetHealthy sets the mock health status
func (m *MockDB) SetHealthy(healthy bool) {
	m.healthy = healthy
}
