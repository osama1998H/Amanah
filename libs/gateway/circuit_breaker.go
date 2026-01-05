package gateway

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota // Circuit is closed, requests flow normally
	StateOpen                       // Circuit is open, requests are blocked
	StateHalfOpen                   // Circuit is half-open, testing if service recovered
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitConfig holds circuit breaker configuration
type CircuitConfig struct {
	Name             string        `json:"name"`
	MaxFailures      int           `json:"max_failures"`       // Failures before opening
	Timeout          time.Duration `json:"timeout"`            // Time before trying half-open
	HalfOpenRequests int           `json:"half_open_requests"` // Requests to test in half-open
	SuccessThreshold int           `json:"success_threshold"`  // Successes needed to close
}

// DefaultCircuitConfig returns default circuit breaker configuration
func DefaultCircuitConfig() *CircuitConfig {
	return &CircuitConfig{
		MaxFailures:      5,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 3,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitConfig

	mu               sync.RWMutex
	state            CircuitState
	failures         int
	successes        int
	lastFailure      time.Time
	halfOpenRequests int
	stateChangedAt   time.Time

	// Callbacks
	onStateChange func(from, to CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitConfig()
	}
	return &CircuitBreaker{
		config:         config,
		state:          StateClosed,
		stateChangedAt: time.Now(),
	}
}

// Allow checks if a request should be allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenRequests = 1
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests < cb.config.HalfOpenRequests {
			cb.halfOpenRequests++
			return true
		}
		return false
	}

	return false
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		// Check if we should close the circuit
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failures++
		// Check if we should open the circuit
		if cb.failures >= cb.config.MaxFailures {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open state opens the circuit again
		cb.transitionTo(StateOpen)
	}
}

// transitionTo changes the circuit state
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.stateChangedAt = time.Now()

	// Reset counters on state change
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0

	// Notify callback if set
	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// LastFailure returns the time of the last failure
func (cb *CircuitBreaker) LastFailure() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastFailure
}

// StateChangedAt returns when the state last changed
func (cb *CircuitBreaker) StateChangedAt() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.stateChangedAt
}

// OnStateChange sets a callback for state changes
func (cb *CircuitBreaker) OnStateChange(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.stateChangedAt = time.Now()
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitStats{
		State:            cb.state.String(),
		Failures:         cb.failures,
		Successes:        cb.successes,
		LastFailure:      cb.lastFailure,
		StateChangedAt:   cb.stateChangedAt,
		HalfOpenRequests: cb.halfOpenRequests,
	}
}

// CircuitStats holds circuit breaker statistics
type CircuitStats struct {
	State            string    `json:"state"`
	Failures         int       `json:"failures"`
	Successes        int       `json:"successes"`
	LastFailure      time.Time `json:"last_failure,omitempty"`
	StateChangedAt   time.Time `json:"state_changed_at"`
	HalfOpenRequests int       `json:"half_open_requests"`
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new circuit breaker registry
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get returns or creates a circuit breaker for a service
func (r *CircuitBreakerRegistry) Get(name string, config *CircuitConfig) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists := r.breakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(config)
	r.breakers[name] = cb
	return cb
}

// Stats returns statistics for all circuit breakers
func (r *CircuitBreakerRegistry) Stats() map[string]CircuitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]CircuitStats)
	for name, cb := range r.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

// ResetAll resets all circuit breakers
func (r *CircuitBreakerRegistry) ResetAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cb := range r.breakers {
		cb.Reset()
	}
}
