package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("key not found")
	ErrExpired      = errors.New("key expired")
	ErrInvalidValue = errors.New("invalid value")
)

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	Close() error
}

// item represents a cached item with expiration
type item struct {
	Value     []byte
	ExpiresAt time.Time
}

// InMemoryCache provides an in-memory cache implementation
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]*item
	stop  chan struct{}
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache() *InMemoryCache {
	c := &InMemoryCache{
		items: make(map[string]*item),
		stop:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a value from the cache
func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, ErrNotFound
	}

	if time.Now().After(item.ExpiresAt) {
		return nil, ErrExpired
	}

	return item.Value, nil
}

// Set stores a value in the cache
func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &item{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a value from the cache
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Exists checks if a key exists in the cache
func (c *InMemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false, nil
	}

	if time.Now().After(item.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear removes all items from the cache
func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*item)
	return nil
}

// Close stops the cache cleanup goroutine
func (c *InMemoryCache) Close() error {
	close(c.stop)
	return nil
}

// cleanup periodically removes expired items
func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.items {
				if now.After(item.ExpiresAt) {
					delete(c.items, key)
				}
			}
			c.mu.Unlock()
		case <-c.stop:
			return
		}
	}
}

// GetJSON retrieves and unmarshals a JSON value
func GetJSON[T any](ctx context.Context, c Cache, key string) (T, error) {
	var result T
	data, err := c.Get(ctx, key)
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, ErrInvalidValue
	}

	return result, nil
}

// SetJSON marshals and stores a JSON value
func SetJSON[T any](ctx context.Context, c Cache, key string, value T, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return ErrInvalidValue
	}

	return c.Set(ctx, key, data, ttl)
}

// CacheKey generates a namespaced cache key
func CacheKey(namespace string, parts ...string) string {
	key := namespace
	for _, part := range parts {
		key += ":" + part
	}
	return key
}

// WithPrefix creates a prefixed cache wrapper
type PrefixedCache struct {
	cache  Cache
	prefix string
}

// NewPrefixedCache creates a cache with a key prefix
func NewPrefixedCache(cache Cache, prefix string) *PrefixedCache {
	return &PrefixedCache{
		cache:  cache,
		prefix: prefix,
	}
}

// Get retrieves a value with the prefix
func (p *PrefixedCache) Get(ctx context.Context, key string) ([]byte, error) {
	return p.cache.Get(ctx, p.prefix+":"+key)
}

// Set stores a value with the prefix
func (p *PrefixedCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return p.cache.Set(ctx, p.prefix+":"+key, value, ttl)
}

// Delete removes a value with the prefix
func (p *PrefixedCache) Delete(ctx context.Context, key string) error {
	return p.cache.Delete(ctx, p.prefix+":"+key)
}

// Exists checks if a key exists with the prefix
func (p *PrefixedCache) Exists(ctx context.Context, key string) (bool, error) {
	return p.cache.Exists(ctx, p.prefix+":"+key)
}

// Clear is not supported for prefixed cache
func (p *PrefixedCache) Clear(ctx context.Context) error {
	return errors.New("clear not supported for prefixed cache")
}

// Close closes the underlying cache
func (p *PrefixedCache) Close() error {
	return p.cache.Close()
}
