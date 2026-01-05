package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// API Key Errors
var (
	ErrAPIKeyNotFound  = errors.New("api key not found")
	ErrAPIKeyExpired   = errors.New("api key expired")
	ErrAPIKeyDisabled  = errors.New("api key disabled")
	ErrAPIKeyRateLimit = errors.New("api key rate limit exceeded")
)

// APIKey represents an API key with metadata
type APIKey struct {
	ID          string            `json:"id"`
	KeyHash     string            `json:"-"` // Hashed key, never expose
	Prefix      string            `json:"prefix"` // First 8 chars for identification
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	ServiceID   string            `json:"service_id,omitempty"` // Associated service
	TenantID    string            `json:"tenant_id,omitempty"`  // Multi-tenant support
	Roles       []string          `json:"roles"`
	Scopes      []string          `json:"scopes,omitempty"`
	RateLimit   int               `json:"rate_limit"`    // Requests per minute, 0 = unlimited
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsActive    bool              `json:"is_active"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// APIKeyManager manages API keys
type APIKeyManager struct {
	mu         sync.RWMutex
	keys       map[string]*APIKey // keyHash -> APIKey
	prefixMap  map[string]string  // prefix -> keyHash (for quick lookup)
	rateLimits map[string]*rateLimitBucket
}

// rateLimitBucket tracks rate limiting per key
type rateLimitBucket struct {
	count     int
	resetTime time.Time
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys:       make(map[string]*APIKey),
		prefixMap:  make(map[string]string),
		rateLimits: make(map[string]*rateLimitBucket),
	}
}

// GenerateAPIKey generates a new API key and returns the raw key (only shown once)
func (m *APIKeyManager) GenerateAPIKey(name, serviceID, tenantID string, roles []string, rateLimit int, expiresIn *time.Duration) (rawKey string, apiKey *APIKey, err error) {
	// Generate raw key
	rawKey, err = randomString(32)
	if err != nil {
		return "", nil, err
	}

	// Hash the key for storage
	keyHash := hashAPIKey(rawKey)
	prefix := rawKey[:8]

	id, err := randomString(16)
	if err != nil {
		return "", nil, err
	}

	now := time.Now()
	apiKey = &APIKey{
		ID:        id,
		KeyHash:   keyHash,
		Prefix:    prefix,
		Name:      name,
		ServiceID: serviceID,
		TenantID:  tenantID,
		Roles:     roles,
		RateLimit: rateLimit,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if expiresIn != nil {
		exp := now.Add(*expiresIn)
		apiKey.ExpiresAt = &exp
	}

	m.mu.Lock()
	m.keys[keyHash] = apiKey
	m.prefixMap[prefix] = keyHash
	m.mu.Unlock()

	return rawKey, apiKey, nil
}

// ValidateAPIKey validates an API key and returns the key metadata
func (m *APIKeyManager) ValidateAPIKey(rawKey string) (*APIKey, error) {
	keyHash := hashAPIKey(rawKey)

	m.mu.RLock()
	apiKey, exists := m.keys[keyHash]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrAPIKeyNotFound
	}

	if !apiKey.IsActive {
		return nil, ErrAPIKeyDisabled
	}

	if apiKey.IsExpired() {
		return nil, ErrAPIKeyExpired
	}

	// Check rate limit
	if apiKey.RateLimit > 0 {
		if err := m.checkRateLimit(keyHash, apiKey.RateLimit); err != nil {
			return nil, err
		}
	}

	// Update last used time
	m.mu.Lock()
	now := time.Now()
	apiKey.LastUsedAt = &now
	m.mu.Unlock()

	return apiKey, nil
}

// checkRateLimit checks and updates rate limit for a key
func (m *APIKeyManager) checkRateLimit(keyHash string, limit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	bucket, exists := m.rateLimits[keyHash]

	if !exists || now.After(bucket.resetTime) {
		// Reset bucket
		m.rateLimits[keyHash] = &rateLimitBucket{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return nil
	}

	if bucket.count >= limit {
		return ErrAPIKeyRateLimit
	}

	bucket.count++
	return nil
}

// GetAPIKeyByPrefix finds an API key by its prefix
func (m *APIKeyManager) GetAPIKeyByPrefix(prefix string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyHash, exists := m.prefixMap[prefix]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}

	apiKey, exists := m.keys[keyHash]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}

	return apiKey, nil
}

// GetAPIKeyByID finds an API key by its ID
func (m *APIKeyManager) GetAPIKeyByID(id string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, apiKey := range m.keys {
		if apiKey.ID == id {
			return apiKey, nil
		}
	}

	return nil, ErrAPIKeyNotFound
}

// ListAPIKeys returns all API keys (without sensitive data)
func (m *APIKeyManager) ListAPIKeys() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	return keys
}

// ListAPIKeysByService returns all API keys for a service
func (m *APIKeyManager) ListAPIKeysByService(serviceID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0)
	for _, key := range m.keys {
		if key.ServiceID == serviceID {
			keys = append(keys, key)
		}
	}
	return keys
}

// ListAPIKeysByTenant returns all API keys for a tenant
func (m *APIKeyManager) ListAPIKeysByTenant(tenantID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0)
	for _, key := range m.keys {
		if key.TenantID == tenantID {
			keys = append(keys, key)
		}
	}
	return keys
}

// RevokeAPIKey disables an API key
func (m *APIKeyManager) RevokeAPIKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, apiKey := range m.keys {
		if apiKey.ID == id {
			apiKey.IsActive = false
			apiKey.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrAPIKeyNotFound
}

// DeleteAPIKey permanently removes an API key
func (m *APIKeyManager) DeleteAPIKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for keyHash, apiKey := range m.keys {
		if apiKey.ID == id {
			delete(m.prefixMap, apiKey.Prefix)
			delete(m.keys, keyHash)
			delete(m.rateLimits, keyHash)
			return nil
		}
	}

	return ErrAPIKeyNotFound
}

// RotateAPIKey creates a new key while optionally keeping the old one active for a grace period
func (m *APIKeyManager) RotateAPIKey(id string, gracePeriod *time.Duration) (newRawKey string, newAPIKey *APIKey, err error) {
	oldKey, err := m.GetAPIKeyByID(id)
	if err != nil {
		return "", nil, err
	}

	// Generate new key with same configuration
	var expiresIn *time.Duration
	if oldKey.ExpiresAt != nil {
		remaining := time.Until(*oldKey.ExpiresAt)
		expiresIn = &remaining
	}

	newRawKey, newAPIKey, err = m.GenerateAPIKey(
		oldKey.Name+" (rotated)",
		oldKey.ServiceID,
		oldKey.TenantID,
		oldKey.Roles,
		oldKey.RateLimit,
		expiresIn,
	)
	if err != nil {
		return "", nil, err
	}

	// Copy metadata
	newAPIKey.Scopes = oldKey.Scopes
	newAPIKey.Metadata = oldKey.Metadata

	// Handle old key
	if gracePeriod != nil && *gracePeriod > 0 {
		// Set expiration for old key
		m.mu.Lock()
		exp := time.Now().Add(*gracePeriod)
		oldKey.ExpiresAt = &exp
		oldKey.UpdatedAt = time.Now()
		m.mu.Unlock()
	} else {
		// Immediately revoke old key
		m.RevokeAPIKey(id)
	}

	return newRawKey, newAPIKey, nil
}

// UpdateAPIKeyRoles updates the roles for an API key
func (m *APIKeyManager) UpdateAPIKeyRoles(id string, roles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, apiKey := range m.keys {
		if apiKey.ID == id {
			apiKey.Roles = roles
			apiKey.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrAPIKeyNotFound
}

// UpdateAPIKeyRateLimit updates the rate limit for an API key
func (m *APIKeyManager) UpdateAPIKeyRateLimit(id string, rateLimit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, apiKey := range m.keys {
		if apiKey.ID == id {
			apiKey.RateLimit = rateLimit
			apiKey.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrAPIKeyNotFound
}

// hashAPIKey creates a SHA-256 hash of the API key
func hashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// ConstantTimeCompare performs constant-time string comparison
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ExtractAPIKeyFromHeader extracts API key from X-API-Key header
func ExtractAPIKeyFromHeader(header string) string {
	return header
}
