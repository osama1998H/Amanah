package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Retry errors
var (
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
	ErrContextCanceled    = errors.New("context canceled")
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	Multiplier      float64       // Multiplier for exponential backoff
	Jitter          float64       // Jitter factor (0-1) to add randomness
	RetryIf         func(error) bool // Function to determine if error is retryable
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryIf:      IsRetryable,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return ErrContextCanceled
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if config.RetryIf != nil && !config.RetryIf(err) {
			return err
		}

		// Check if we've exhausted retries
		if attempt >= config.MaxRetries {
			break
		}

		// Calculate delay with jitter
		jitteredDelay := addJitter(delay, config.Jitter)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ErrContextCanceled
		case <-time.After(jitteredDelay):
		}

		// Increase delay for next retry
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return errors.Join(ErrMaxRetriesExceeded, lastErr)
}

// RetryWithResult executes a function that returns a value with retry logic
func RetryWithResult[T any](ctx context.Context, config *RetryConfig, fn func() (T, error)) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var zero T
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ErrContextCanceled
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if config.RetryIf != nil && !config.RetryIf(err) {
			return zero, err
		}

		if attempt >= config.MaxRetries {
			break
		}

		jitteredDelay := addJitter(delay, config.Jitter)

		select {
		case <-ctx.Done():
			return zero, ErrContextCanceled
		case <-time.After(jitteredDelay):
		}

		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return zero, errors.Join(ErrMaxRetriesExceeded, lastErr)
}

// addJitter adds random jitter to a duration
func addJitter(d time.Duration, factor float64) time.Duration {
	if factor <= 0 {
		return d
	}

	jitter := float64(d) * factor * (rand.Float64()*2 - 1) // -factor to +factor
	return time.Duration(float64(d) + jitter)
}

// RetryableFunc wraps a function with retry logic
type RetryableFunc struct {
	config *RetryConfig
}

// NewRetryableFunc creates a new retryable function wrapper
func NewRetryableFunc(config *RetryConfig) *RetryableFunc {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryableFunc{config: config}
}

// Do executes the function with retry
func (r *RetryableFunc) Do(ctx context.Context, fn func() error) error {
	return Retry(ctx, r.config, fn)
}

// DoWithResult executes a function that returns a value
func (r *RetryableFunc) DoWithResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	return RetryWithResult(ctx, r.config, fn)
}

// ExponentialBackoff calculates the delay for exponential backoff
func ExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration, multiplier float64) time.Duration {
	delay := float64(baseDelay) * math.Pow(multiplier, float64(attempt))
	if delay > float64(maxDelay) {
		return maxDelay
	}
	return time.Duration(delay)
}

// LinearBackoff calculates the delay for linear backoff
func LinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := baseDelay * time.Duration(attempt+1)
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// ConstantBackoff returns a constant delay
func ConstantBackoff(delay time.Duration) time.Duration {
	return delay
}

// RetryPolicy defines a retry policy
type RetryPolicy struct {
	MaxAttempts     int
	BackoffStrategy func(attempt int) time.Duration
	ShouldRetry     func(error) bool
}

// DefaultRetryPolicy returns a default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 3,
		BackoffStrategy: func(attempt int) time.Duration {
			return ExponentialBackoff(attempt, 100*time.Millisecond, 30*time.Second, 2.0)
		},
		ShouldRetry: IsRetryable,
	}
}

// Execute executes a function with the retry policy
func (p *RetryPolicy) Execute(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ErrContextCanceled
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if p.ShouldRetry != nil && !p.ShouldRetry(err) {
			return err
		}

		if attempt < p.MaxAttempts-1 {
			delay := p.BackoffStrategy(attempt)
			select {
			case <-ctx.Done():
				return ErrContextCanceled
			case <-time.After(delay):
			}
		}
	}

	return errors.Join(ErrMaxRetriesExceeded, lastErr)
}
