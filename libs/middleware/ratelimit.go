package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Rate Limit Errors
var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// RateLimitConfig holds rate limiter configuration
type RateLimitConfig struct {
	RequestsPerSecond float64       // Requests allowed per second
	BurstSize         int           // Maximum burst size
	CleanupInterval   time.Duration // Cleanup interval for expired entries
	KeyFunc           func(*http.Request) string // Function to extract key from request
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		CleanupInterval:   time.Minute,
		KeyFunc:           DefaultKeyFunc,
	}
}

// DefaultKeyFunc extracts client IP as the rate limit key
func DefaultKeyFunc(r *http.Request) string {
	// Try X-Forwarded-For first (behind proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// TokenBucket implements the token bucket algorithm
type TokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(maxTokens float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate

	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}

	tb.lastRefill = now
}

// Tokens returns the current number of tokens
func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// RateLimiter manages rate limiting for multiple clients
type RateLimiter struct {
	config  *RateLimitConfig
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
	stop    chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:  config,
		buckets: make(map[string]*TokenBucket),
		stop:    make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = NewTokenBucket(float64(rl.config.BurstSize), rl.config.RequestsPerSecond)
		rl.buckets[key] = bucket
	}

	rl.mu.Unlock()

	return bucket.Allow()
}

// cleanup removes stale buckets periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			// Remove buckets that have been full for a while (idle clients)
			for key, bucket := range rl.buckets {
				if bucket.Tokens() >= bucket.maxTokens {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

// Close stops the rate limiter
func (rl *RateLimiter) Close() {
	close(rl.stop)
}

// Middleware returns an HTTP middleware for rate limiting
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.config.KeyFunc(r)

			if !rl.Allow(key) {
				rl.mu.RLock()
				bucket := rl.buckets[key]
				rl.mu.RUnlock()

				// Set rate limit headers
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.BurstSize))
				w.Header().Set("X-RateLimit-Remaining", "0")
				if bucket != nil {
					// Time until a token is available
					waitTime := (1 - bucket.Tokens()) / rl.config.RequestsPerSecond
					w.Header().Set("Retry-After", strconv.Itoa(int(waitTime)+1))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "rate_limit_exceeded",
					"message": "Too many requests. Please retry later.",
				})
				return
			}

			// Set rate limit headers for successful requests
			rl.mu.RLock()
			bucket := rl.buckets[key]
			rl.mu.RUnlock()

			if bucket != nil {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.BurstSize))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(bucket.Tokens())))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SlidingWindowLimiter implements sliding window rate limiting
type SlidingWindowLimiter struct {
	config   *SlidingWindowConfig
	windows  map[string]*slidingWindow
	mu       sync.RWMutex
	stop     chan struct{}
}

// SlidingWindowConfig holds sliding window configuration
type SlidingWindowConfig struct {
	WindowSize      time.Duration
	MaxRequests     int
	CleanupInterval time.Duration
	KeyFunc         func(*http.Request) string
}

// DefaultSlidingWindowConfig returns default sliding window configuration
func DefaultSlidingWindowConfig() *SlidingWindowConfig {
	return &SlidingWindowConfig{
		WindowSize:      time.Minute,
		MaxRequests:     60,
		CleanupInterval: time.Minute * 5,
		KeyFunc:         DefaultKeyFunc,
	}
}

// slidingWindow tracks requests in a time window
type slidingWindow struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindowLimiter creates a new sliding window limiter
func NewSlidingWindowLimiter(config *SlidingWindowConfig) *SlidingWindowLimiter {
	if config == nil {
		config = DefaultSlidingWindowConfig()
	}

	sw := &SlidingWindowLimiter{
		config:  config,
		windows: make(map[string]*slidingWindow),
		stop:    make(chan struct{}),
	}

	go sw.cleanup()

	return sw
}

// Allow checks if a request should be allowed
func (sw *SlidingWindowLimiter) Allow(key string) bool {
	sw.mu.Lock()
	window, exists := sw.windows[key]
	if !exists {
		window = &slidingWindow{requests: make([]time.Time, 0)}
		sw.windows[key] = window
	}
	sw.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.config.WindowSize)

	// Remove old requests
	validRequests := make([]time.Time, 0)
	for _, t := range window.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	window.requests = validRequests

	// Check if we're under the limit
	if len(window.requests) >= sw.config.MaxRequests {
		return false
	}

	// Add new request
	window.requests = append(window.requests, now)
	return true
}

// cleanup removes stale windows
func (sw *SlidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(sw.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sw.mu.Lock()
			cutoff := time.Now().Add(-sw.config.WindowSize * 2)
			for key, window := range sw.windows {
				window.mu.Lock()
				if len(window.requests) == 0 {
					delete(sw.windows, key)
				} else if window.requests[len(window.requests)-1].Before(cutoff) {
					delete(sw.windows, key)
				}
				window.mu.Unlock()
			}
			sw.mu.Unlock()
		case <-sw.stop:
			return
		}
	}
}

// Close stops the limiter
func (sw *SlidingWindowLimiter) Close() {
	close(sw.stop)
}

// Middleware returns an HTTP middleware for sliding window rate limiting
func (sw *SlidingWindowLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := sw.config.KeyFunc(r)

			if !sw.Allow(key) {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(sw.config.MaxRequests))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Window", sw.config.WindowSize.String())

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "rate_limit_exceeded",
					"message": "Too many requests. Please retry later.",
				})
				return
			}

			// Get remaining requests
			sw.mu.RLock()
			window := sw.windows[key]
			sw.mu.RUnlock()

			if window != nil {
				window.mu.Lock()
				remaining := sw.config.MaxRequests - len(window.requests)
				window.mu.Unlock()
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(sw.config.MaxRequests))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EndpointRateLimiter allows different rate limits per endpoint
type EndpointRateLimiter struct {
	defaultLimiter *RateLimiter
	endpointLimits map[string]*RateLimiter
	mu             sync.RWMutex
}

// NewEndpointRateLimiter creates a new endpoint-specific rate limiter
func NewEndpointRateLimiter(defaultConfig *RateLimitConfig) *EndpointRateLimiter {
	return &EndpointRateLimiter{
		defaultLimiter: NewRateLimiter(defaultConfig),
		endpointLimits: make(map[string]*RateLimiter),
	}
}

// SetEndpointLimit sets a specific rate limit for an endpoint
func (e *EndpointRateLimiter) SetEndpointLimit(endpoint string, config *RateLimitConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.endpointLimits[endpoint] = NewRateLimiter(config)
}

// Middleware returns the middleware
func (e *EndpointRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint := r.URL.Path

			e.mu.RLock()
			limiter, exists := e.endpointLimits[endpoint]
			if !exists {
				limiter = e.defaultLimiter
			}
			e.mu.RUnlock()

			key := limiter.config.KeyFunc(r)
			if !limiter.Allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "rate_limit_exceeded",
					"message": "Too many requests to this endpoint. Please retry later.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Close closes all rate limiters
func (e *EndpointRateLimiter) Close() {
	e.defaultLimiter.Close()
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, limiter := range e.endpointLimits {
		limiter.Close()
	}
}
