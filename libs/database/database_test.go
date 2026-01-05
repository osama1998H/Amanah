package database

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", cfg.Port)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("Expected max open conns 25, got %d", cfg.MaxOpenConns)
	}
}

func TestConfigDSN(t *testing.T) {
	cfg := &Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}

	dsn := cfg.DSN()
	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	if dsn != expected {
		t.Errorf("Expected DSN %s, got %s", expected, dsn)
	}
}

func TestMockDB(t *testing.T) {
	mock := NewMockDB()

	// Test Set and Get
	mock.Set("users", "user1", map[string]string{"name": "John"})
	val, ok := mock.Get("users", "user1")
	if !ok {
		t.Error("Expected to find user1")
	}
	if val.(map[string]string)["name"] != "John" {
		t.Error("Expected name to be John")
	}

	// Test non-existent key
	_, ok = mock.Get("users", "user2")
	if ok {
		t.Error("Expected not to find user2")
	}

	// Test Delete
	mock.Delete("users", "user1")
	_, ok = mock.Get("users", "user1")
	if ok {
		t.Error("Expected user1 to be deleted")
	}
}

func TestMockDBHealthcheck(t *testing.T) {
	mock := NewMockDB()
	ctx := context.Background()

	// Test healthy
	if err := mock.Healthcheck(ctx); err != nil {
		t.Errorf("Expected healthy, got error: %v", err)
	}

	// Test unhealthy
	mock.SetHealthy(false)
	if err := mock.Healthcheck(ctx); err == nil {
		t.Error("Expected error for unhealthy mock")
	}
}

func TestMockDBConcurrency(t *testing.T) {
	mock := NewMockDB()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 100; i++ {
		go func(id int) {
			key := "key" + string(rune('0'+id%10))
			mock.Set("test", key, id)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(id int) {
			key := "key" + string(rune('0'+id%10))
			mock.Get("test", key)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"NotFound", ErrNotFound, "record not found"},
		{"DuplicateKey", ErrDuplicateKey, "duplicate key"},
		{"ConnectionFailed", ErrConnectionFailed, "database connection failed"},
		{"TransactionFailed", ErrTransactionFailed, "transaction failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("Expected error message %s, got %s", tt.msg, tt.err.Error())
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Expected conn max lifetime 5m, got %v", cfg.ConnMaxLifetime)
	}

	if cfg.ConnMaxIdleTime != 1*time.Minute {
		t.Errorf("Expected conn max idle time 1m, got %v", cfg.ConnMaxIdleTime)
	}
}
