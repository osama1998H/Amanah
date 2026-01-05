package auth

import (
	"testing"
	"time"
)

func TestAPIKeyManagerGenerate(t *testing.T) {
	manager := NewAPIKeyManager()

	rawKey, apiKey, err := manager.GenerateAPIKey("Test Key", "service-1", "tenant-1", []string{"user"}, 100, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if rawKey == "" {
		t.Error("Raw key should not be empty")
	}

	if apiKey.ID == "" {
		t.Error("API key ID should not be empty")
	}

	if apiKey.Name != "Test Key" {
		t.Errorf("Expected name 'Test Key', got '%s'", apiKey.Name)
	}

	if apiKey.ServiceID != "service-1" {
		t.Errorf("Expected service ID 'service-1', got '%s'", apiKey.ServiceID)
	}

	if apiKey.TenantID != "tenant-1" {
		t.Errorf("Expected tenant ID 'tenant-1', got '%s'", apiKey.TenantID)
	}

	if len(apiKey.Roles) != 1 || apiKey.Roles[0] != "user" {
		t.Errorf("Expected roles [user], got %v", apiKey.Roles)
	}

	if apiKey.RateLimit != 100 {
		t.Errorf("Expected rate limit 100, got %d", apiKey.RateLimit)
	}

	if !apiKey.IsActive {
		t.Error("API key should be active by default")
	}

	if apiKey.Prefix != rawKey[:8] {
		t.Error("Prefix should match first 8 characters of raw key")
	}
}

func TestAPIKeyManagerValidate(t *testing.T) {
	manager := NewAPIKeyManager()

	rawKey, _, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Validate the key
	key, err := manager.ValidateAPIKey(rawKey)
	if err != nil {
		t.Fatalf("Failed to validate API key: %v", err)
	}

	if key.Name != "Test" {
		t.Errorf("Expected name 'Test', got '%s'", key.Name)
	}

	if key.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after validation")
	}
}

func TestAPIKeyManagerValidateInvalid(t *testing.T) {
	manager := NewAPIKeyManager()

	_, err := manager.ValidateAPIKey("invalid-key")
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound, got: %v", err)
	}
}

func TestAPIKeyManagerExpiration(t *testing.T) {
	manager := NewAPIKeyManager()

	// Create key with very short expiration
	expireIn := time.Millisecond
	rawKey, _, err := manager.GenerateAPIKey("Expiring", "", "", []string{"user"}, 0, &expireIn)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired
	_, err = manager.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyExpired {
		t.Errorf("Expected ErrAPIKeyExpired, got: %v", err)
	}
}

func TestAPIKeyManagerRevoke(t *testing.T) {
	manager := NewAPIKeyManager()

	rawKey, apiKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Revoke the key
	err = manager.RevokeAPIKey(apiKey.ID)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	// Should be disabled
	_, err = manager.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyDisabled {
		t.Errorf("Expected ErrAPIKeyDisabled, got: %v", err)
	}
}

func TestAPIKeyManagerDelete(t *testing.T) {
	manager := NewAPIKeyManager()

	rawKey, apiKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Delete the key
	err = manager.DeleteAPIKey(apiKey.ID)
	if err != nil {
		t.Fatalf("Failed to delete API key: %v", err)
	}

	// Should not be found
	_, err = manager.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound, got: %v", err)
	}

	// Delete non-existent key
	err = manager.DeleteAPIKey("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound, got: %v", err)
	}
}

func TestAPIKeyManagerRateLimit(t *testing.T) {
	manager := NewAPIKeyManager()

	// Create key with rate limit of 2 per minute
	rawKey, _, err := manager.GenerateAPIKey("Limited", "", "", []string{"user"}, 2, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// First two requests should succeed
	_, err = manager.ValidateAPIKey(rawKey)
	if err != nil {
		t.Errorf("First request should succeed: %v", err)
	}

	_, err = manager.ValidateAPIKey(rawKey)
	if err != nil {
		t.Errorf("Second request should succeed: %v", err)
	}

	// Third request should be rate limited
	_, err = manager.ValidateAPIKey(rawKey)
	if err != ErrAPIKeyRateLimit {
		t.Errorf("Expected ErrAPIKeyRateLimit, got: %v", err)
	}
}

func TestAPIKeyManagerGetByPrefix(t *testing.T) {
	manager := NewAPIKeyManager()

	rawKey, _, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	prefix := rawKey[:8]

	key, err := manager.GetAPIKeyByPrefix(prefix)
	if err != nil {
		t.Fatalf("Failed to get key by prefix: %v", err)
	}

	if key.Prefix != prefix {
		t.Errorf("Expected prefix %s, got %s", prefix, key.Prefix)
	}
}

func TestAPIKeyManagerGetByID(t *testing.T) {
	manager := NewAPIKeyManager()

	_, apiKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	key, err := manager.GetAPIKeyByID(apiKey.ID)
	if err != nil {
		t.Fatalf("Failed to get key by ID: %v", err)
	}

	if key.ID != apiKey.ID {
		t.Errorf("Expected ID %s, got %s", apiKey.ID, key.ID)
	}

	// Non-existent ID
	_, err = manager.GetAPIKeyByID("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound, got: %v", err)
	}
}

func TestAPIKeyManagerListByService(t *testing.T) {
	manager := NewAPIKeyManager()

	manager.GenerateAPIKey("Key1", "service-a", "", []string{"user"}, 0, nil)
	manager.GenerateAPIKey("Key2", "service-a", "", []string{"user"}, 0, nil)
	manager.GenerateAPIKey("Key3", "service-b", "", []string{"user"}, 0, nil)

	keys := manager.ListAPIKeysByService("service-a")
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys for service-a, got %d", len(keys))
	}

	keys = manager.ListAPIKeysByService("service-b")
	if len(keys) != 1 {
		t.Errorf("Expected 1 key for service-b, got %d", len(keys))
	}
}

func TestAPIKeyManagerListByTenant(t *testing.T) {
	manager := NewAPIKeyManager()

	manager.GenerateAPIKey("Key1", "", "tenant-a", []string{"user"}, 0, nil)
	manager.GenerateAPIKey("Key2", "", "tenant-a", []string{"user"}, 0, nil)
	manager.GenerateAPIKey("Key3", "", "tenant-b", []string{"user"}, 0, nil)

	keys := manager.ListAPIKeysByTenant("tenant-a")
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys for tenant-a, got %d", len(keys))
	}
}

func TestAPIKeyManagerRotate(t *testing.T) {
	manager := NewAPIKeyManager()

	oldRawKey, oldKey, err := manager.GenerateAPIKey("Test", "svc", "tenant", []string{"user"}, 50, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Rotate with grace period
	gracePeriod := time.Hour
	newRawKey, newKey, err := manager.RotateAPIKey(oldKey.ID, &gracePeriod)
	if err != nil {
		t.Fatalf("Failed to rotate API key: %v", err)
	}

	if newRawKey == oldRawKey {
		t.Error("New key should be different from old key")
	}

	// Old key should still work during grace period
	_, err = manager.ValidateAPIKey(oldRawKey)
	if err != nil {
		t.Errorf("Old key should still work during grace period: %v", err)
	}

	// New key should work
	_, err = manager.ValidateAPIKey(newRawKey)
	if err != nil {
		t.Errorf("New key should work: %v", err)
	}

	// New key should have same service/tenant
	if newKey.ServiceID != "svc" {
		t.Errorf("Expected service ID 'svc', got '%s'", newKey.ServiceID)
	}
}

func TestAPIKeyManagerRotateImmediateRevoke(t *testing.T) {
	manager := NewAPIKeyManager()

	oldRawKey, oldKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Rotate without grace period
	newRawKey, _, err := manager.RotateAPIKey(oldKey.ID, nil)
	if err != nil {
		t.Fatalf("Failed to rotate API key: %v", err)
	}

	// Old key should be revoked immediately
	_, err = manager.ValidateAPIKey(oldRawKey)
	if err != ErrAPIKeyDisabled {
		t.Errorf("Old key should be disabled, got: %v", err)
	}

	// New key should work
	_, err = manager.ValidateAPIKey(newRawKey)
	if err != nil {
		t.Errorf("New key should work: %v", err)
	}
}

func TestAPIKeyManagerUpdateRoles(t *testing.T) {
	manager := NewAPIKeyManager()

	_, apiKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 0, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Update roles
	err = manager.UpdateAPIKeyRoles(apiKey.ID, []string{"admin", "user"})
	if err != nil {
		t.Fatalf("Failed to update roles: %v", err)
	}

	key, _ := manager.GetAPIKeyByID(apiKey.ID)
	if len(key.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(key.Roles))
	}

	// Update non-existent key
	err = manager.UpdateAPIKeyRoles("nonexistent", []string{"user"})
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound, got: %v", err)
	}
}

func TestAPIKeyManagerUpdateRateLimit(t *testing.T) {
	manager := NewAPIKeyManager()

	_, apiKey, err := manager.GenerateAPIKey("Test", "", "", []string{"user"}, 100, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Update rate limit
	err = manager.UpdateAPIKeyRateLimit(apiKey.ID, 200)
	if err != nil {
		t.Fatalf("Failed to update rate limit: %v", err)
	}

	key, _ := manager.GetAPIKeyByID(apiKey.ID)
	if key.RateLimit != 200 {
		t.Errorf("Expected rate limit 200, got %d", key.RateLimit)
	}
}

func TestAPIKeyIsExpired(t *testing.T) {
	// No expiration
	key1 := &APIKey{}
	if key1.IsExpired() {
		t.Error("Key without expiration should not be expired")
	}

	// Future expiration
	future := time.Now().Add(time.Hour)
	key2 := &APIKey{ExpiresAt: &future}
	if key2.IsExpired() {
		t.Error("Key with future expiration should not be expired")
	}

	// Past expiration
	past := time.Now().Add(-time.Hour)
	key3 := &APIKey{ExpiresAt: &past}
	if !key3.IsExpired() {
		t.Error("Key with past expiration should be expired")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	if !ConstantTimeCompare("hello", "hello") {
		t.Error("Same strings should be equal")
	}

	if ConstantTimeCompare("hello", "world") {
		t.Error("Different strings should not be equal")
	}

	if ConstantTimeCompare("hello", "hello!") {
		t.Error("Strings of different length should not be equal")
	}
}
