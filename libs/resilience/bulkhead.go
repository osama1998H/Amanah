package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Bulkhead errors
var (
	ErrBulkheadFull      = errors.New("bulkhead is full")
	ErrBulkheadRejected  = errors.New("request rejected by bulkhead")
	ErrBulkheadTimeout   = errors.New("bulkhead wait timeout")
)

// BulkheadConfig holds bulkhead configuration
type BulkheadConfig struct {
	MaxConcurrent  int           // Maximum concurrent executions
	MaxWaiting     int           // Maximum waiting queue size
	WaitTimeout    time.Duration // Timeout for waiting in queue
}

// DefaultBulkheadConfig returns default bulkhead configuration
func DefaultBulkheadConfig() *BulkheadConfig {
	return &BulkheadConfig{
		MaxConcurrent: 10,
		MaxWaiting:    100,
		WaitTimeout:   30 * time.Second,
	}
}

// Bulkhead implements the bulkhead pattern to limit concurrent executions
type Bulkhead struct {
	config    *BulkheadConfig
	semaphore chan struct{}
	waiting   int
	mu        sync.Mutex

	// Metrics
	activeCount   int
	rejectedCount int64
	successCount  int64
	failureCount  int64
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(config *BulkheadConfig) *Bulkhead {
	if config == nil {
		config = DefaultBulkheadConfig()
	}

	return &Bulkhead{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}
}

// Execute runs a function within the bulkhead
func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
	// Try to acquire immediately
	select {
	case b.semaphore <- struct{}{}:
		b.mu.Lock()
		b.activeCount++
		b.mu.Unlock()
	default:
		// Queue is full, need to wait
		if err := b.wait(ctx); err != nil {
			return err
		}
	}

	// Execute the function
	defer func() {
		<-b.semaphore
		b.mu.Lock()
		b.activeCount--
		b.mu.Unlock()
	}()

	err := fn()
	b.mu.Lock()
	if err != nil {
		b.failureCount++
	} else {
		b.successCount++
	}
	b.mu.Unlock()

	return err
}

// ExecuteWithResult runs a function that returns a value within the bulkhead
func (b *Bulkhead) ExecuteWithResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	select {
	case b.semaphore <- struct{}{}:
		b.mu.Lock()
		b.activeCount++
		b.mu.Unlock()
	default:
		if err := b.wait(ctx); err != nil {
			return nil, err
		}
	}

	defer func() {
		<-b.semaphore
		b.mu.Lock()
		b.activeCount--
		b.mu.Unlock()
	}()

	result, err := fn()
	b.mu.Lock()
	if err != nil {
		b.failureCount++
	} else {
		b.successCount++
	}
	b.mu.Unlock()

	return result, err
}

// wait waits for a slot in the bulkhead
func (b *Bulkhead) wait(ctx context.Context) error {
	b.mu.Lock()
	if b.waiting >= b.config.MaxWaiting {
		b.rejectedCount++
		b.mu.Unlock()
		return ErrBulkheadFull
	}
	b.waiting++
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.waiting--
		b.mu.Unlock()
	}()

	// Create timeout context
	var cancel context.CancelFunc
	if b.config.WaitTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, b.config.WaitTimeout)
		defer cancel()
	}

	select {
	case b.semaphore <- struct{}{}:
		b.mu.Lock()
		b.activeCount++
		b.mu.Unlock()
		return nil
	case <-ctx.Done():
		b.mu.Lock()
		b.rejectedCount++
		b.mu.Unlock()
		if ctx.Err() == context.DeadlineExceeded {
			return ErrBulkheadTimeout
		}
		return ctx.Err()
	}
}

// TryExecute attempts to execute without waiting
func (b *Bulkhead) TryExecute(fn func() error) error {
	select {
	case b.semaphore <- struct{}{}:
		b.mu.Lock()
		b.activeCount++
		b.mu.Unlock()
	default:
		b.mu.Lock()
		b.rejectedCount++
		b.mu.Unlock()
		return ErrBulkheadRejected
	}

	defer func() {
		<-b.semaphore
		b.mu.Lock()
		b.activeCount--
		b.mu.Unlock()
	}()

	err := fn()
	b.mu.Lock()
	if err != nil {
		b.failureCount++
	} else {
		b.successCount++
	}
	b.mu.Unlock()

	return err
}

// Stats returns bulkhead statistics
func (b *Bulkhead) Stats() BulkheadStats {
	b.mu.Lock()
	defer b.mu.Unlock()

	return BulkheadStats{
		MaxConcurrent:  b.config.MaxConcurrent,
		ActiveCount:    b.activeCount,
		WaitingCount:   b.waiting,
		RejectedCount:  b.rejectedCount,
		SuccessCount:   b.successCount,
		FailureCount:   b.failureCount,
		AvailableSlots: b.config.MaxConcurrent - b.activeCount,
	}
}

// BulkheadStats holds bulkhead statistics
type BulkheadStats struct {
	MaxConcurrent  int   `json:"max_concurrent"`
	ActiveCount    int   `json:"active_count"`
	WaitingCount   int   `json:"waiting_count"`
	RejectedCount  int64 `json:"rejected_count"`
	SuccessCount   int64 `json:"success_count"`
	FailureCount   int64 `json:"failure_count"`
	AvailableSlots int   `json:"available_slots"`
}

// SemaphoreBulkhead is a simpler bulkhead using a semaphore
type SemaphoreBulkhead struct {
	semaphore chan struct{}
	timeout   time.Duration
}

// NewSemaphoreBulkhead creates a simple semaphore-based bulkhead
func NewSemaphoreBulkhead(maxConcurrent int, timeout time.Duration) *SemaphoreBulkhead {
	return &SemaphoreBulkhead{
		semaphore: make(chan struct{}, maxConcurrent),
		timeout:   timeout,
	}
}

// Acquire acquires a slot in the bulkhead
func (b *SemaphoreBulkhead) Acquire(ctx context.Context) error {
	if b.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	select {
	case b.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ErrBulkheadTimeout
	}
}

// Release releases a slot in the bulkhead
func (b *SemaphoreBulkhead) Release() {
	select {
	case <-b.semaphore:
	default:
	}
}

// TryAcquire tries to acquire a slot without blocking
func (b *SemaphoreBulkhead) TryAcquire() bool {
	select {
	case b.semaphore <- struct{}{}:
		return true
	default:
		return false
	}
}

// AvailableSlots returns the number of available slots
func (b *SemaphoreBulkhead) AvailableSlots() int {
	return cap(b.semaphore) - len(b.semaphore)
}

// ActiveSlots returns the number of active slots
func (b *SemaphoreBulkhead) ActiveSlots() int {
	return len(b.semaphore)
}
