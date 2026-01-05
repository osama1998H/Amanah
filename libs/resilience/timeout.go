package resilience

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Timeout errors
var (
	ErrTimeout = errors.New("operation timed out")
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	RequestTimeout  time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		RequestTimeout:  30 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// WithTimeout executes a function with a timeout
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return ErrTimeout
		}
		return ctx.Err()
	}
}

// WithTimeoutResult executes a function that returns a value with a timeout
func WithTimeoutResult[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		value T
		err   error
	}

	done := make(chan result, 1)
	go func() {
		v, err := fn(ctx)
		done <- result{v, err}
	}()

	var zero T
	select {
	case r := <-done:
		return r.value, r.err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return zero, ErrTimeout
		}
		return zero, ctx.Err()
	}
}

// TimeoutMiddleware adds a timeout to HTTP requests
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{
				ResponseWriter: w,
				header:         make(http.Header),
			}

			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				// Copy headers and write response
				for k, v := range tw.header {
					w.Header()[k] = v
				}
				if tw.code != 0 {
					w.WriteHeader(tw.code)
				}
				w.Write(tw.body)
			case <-ctx.Done():
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte(`{"error":"GATEWAY_TIMEOUT","message":"Request timed out"}`))
			}
		})
	}
}

// timeoutWriter buffers the response for timeout handling
type timeoutWriter struct {
	http.ResponseWriter
	header http.Header
	code   int
	body   []byte
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.header
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.code = code
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.body = append(tw.body, b...)
	return len(b), nil
}

// Deadline represents a deadline for an operation
type Deadline struct {
	deadline time.Time
	duration time.Duration
}

// NewDeadline creates a new deadline
func NewDeadline(d time.Duration) *Deadline {
	return &Deadline{
		deadline: time.Now().Add(d),
		duration: d,
	}
}

// Remaining returns the remaining time until the deadline
func (d *Deadline) Remaining() time.Duration {
	remaining := time.Until(d.deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Expired returns true if the deadline has passed
func (d *Deadline) Expired() bool {
	return time.Now().After(d.deadline)
}

// Context returns a context with the deadline
func (d *Deadline) Context(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, d.deadline)
}

// Extend extends the deadline by the given duration
func (d *Deadline) Extend(duration time.Duration) {
	d.deadline = d.deadline.Add(duration)
}

// Reset resets the deadline to the original duration from now
func (d *Deadline) Reset() {
	d.deadline = time.Now().Add(d.duration)
}

// TimeoutHTTPClient creates an HTTP client with configured timeouts
func TimeoutHTTPClient(config *TimeoutConfig) *http.Client {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	return &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			ResponseHeaderTimeout: config.ReadTimeout,
			IdleConnTimeout:       config.IdleTimeout,
		},
	}
}
