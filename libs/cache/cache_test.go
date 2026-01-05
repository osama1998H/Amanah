package cache

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryCache(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	// Test Set and Get
	err := c.Set(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(value) != "value1" {
		t.Errorf("Expected value1, got %s", string(value))
	}
}

func TestInMemoryCacheNotFound(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	_, err := c.Get(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestInMemoryCacheExpiration(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	// Set with very short TTL
	err := c.Set(ctx, "expiring", []byte("value"), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	_, err = c.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Expected to find key immediately: %v", err)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	_, err = c.Get(ctx, "expiring")
	if err != ErrExpired {
		t.Errorf("Expected ErrExpired, got %v", err)
	}
}

func TestInMemoryCacheDelete(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	c.Set(ctx, "todelete", []byte("value"), time.Minute)

	err := c.Delete(ctx, "todelete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = c.Get(ctx, "todelete")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound after delete, got %v", err)
	}
}

func TestInMemoryCacheExists(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	// Non-existent key
	exists, err := c.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist")
	}

	// Existing key
	c.Set(ctx, "exists", []byte("value"), time.Minute)
	exists, err = c.Exists(ctx, "exists")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}
}

func TestInMemoryCacheClear(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	c.Set(ctx, "key1", []byte("value1"), time.Minute)
	c.Set(ctx, "key2", []byte("value2"), time.Minute)

	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	_, err = c.Get(ctx, "key1")
	if err != ErrNotFound {
		t.Error("Expected key1 to be cleared")
	}

	_, err = c.Get(ctx, "key2")
	if err != ErrNotFound {
		t.Error("Expected key2 to be cleared")
	}
}

func TestGetSetJSON(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := TestData{Name: "test", Value: 42}

	// Set JSON
	err := SetJSON(ctx, c, "jsonkey", data, time.Minute)
	if err != nil {
		t.Fatalf("SetJSON failed: %v", err)
	}

	// Get JSON
	result, err := GetJSON[TestData](ctx, c, "jsonkey")
	if err != nil {
		t.Fatalf("GetJSON failed: %v", err)
	}

	if result.Name != "test" || result.Value != 42 {
		t.Errorf("Expected {test, 42}, got {%s, %d}", result.Name, result.Value)
	}
}

func TestCacheKey(t *testing.T) {
	key := CacheKey("users", "123", "profile")
	expected := "users:123:profile"
	if key != expected {
		t.Errorf("Expected %s, got %s", expected, key)
	}
}

func TestPrefixedCache(t *testing.T) {
	ctx := context.Background()
	base := NewInMemoryCache()
	defer base.Close()

	prefixed := NewPrefixedCache(base, "myapp")

	// Set with prefix
	err := prefixed.Set(ctx, "key", []byte("value"), time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get with prefix
	value, err := prefixed.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(value) != "value" {
		t.Errorf("Expected value, got %s", string(value))
	}

	// Verify the actual key in base cache
	_, err = base.Get(ctx, "myapp:key")
	if err != nil {
		t.Error("Expected to find prefixed key in base cache")
	}
}

func TestCacheConcurrency(t *testing.T) {
	ctx := context.Background()
	c := NewInMemoryCache()
	defer c.Close()

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 100; i++ {
		go func(id int) {
			key := string(rune('a' + id%26))
			c.Set(ctx, key, []byte("value"), time.Minute)
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(id int) {
			key := string(rune('a' + id%26))
			c.Get(ctx, key)
			done <- true
		}(i)
	}

	// Wait for all operations
	for i := 0; i < 200; i++ {
		<-done
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		err error
		msg string
	}{
		{ErrNotFound, "key not found"},
		{ErrExpired, "key expired"},
		{ErrInvalidValue, "invalid value"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.msg {
			t.Errorf("Expected %s, got %s", tt.msg, tt.err.Error())
		}
	}
}
