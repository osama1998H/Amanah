package resilience

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ==================== Error Tests ====================

func TestErrorCodeHTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeBadRequest, http.StatusBadRequest},
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeMethodNotAllowed, http.StatusMethodNotAllowed},
		{ErrCodeConflict, http.StatusConflict},
		{ErrCodeGone, http.StatusGone},
		{ErrCodeUnprocessable, http.StatusUnprocessableEntity},
		{ErrCodeTooManyRequests, http.StatusTooManyRequests},
		{ErrCodeInternal, http.StatusInternalServerError},
		{ErrCodeNotImplemented, http.StatusNotImplemented},
		{ErrCodeBadGateway, http.StatusBadGateway},
		{ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
		{ErrCodeGatewayTimeout, http.StatusGatewayTimeout},
		{ErrCodeValidation, http.StatusBadRequest},
		{ErrCodeInsufficientFunds, http.StatusUnprocessableEntity},
		{ErrCodeDuplicateEntry, http.StatusConflict},
		{ErrCodeExpired, http.StatusGone},
		{ErrCodeInvalidState, http.StatusUnprocessableEntity},
		{ErrCodeDependencyFailed, http.StatusBadGateway},
		{ErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.expected {
			t.Errorf("ErrorCode(%s).HTTPStatus() = %d, want %d", tt.code, got, tt.expected)
		}
	}
}

func TestAppErrorError(t *testing.T) {
	err := NewError(ErrCodeBadRequest, "invalid input")
	expected := "BAD_REQUEST: invalid input"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	err = err.WithDetails("field X is required")
	expected = "BAD_REQUEST: invalid input - field X is required"
	if err.Error() != expected {
		t.Errorf("Error() with details = %s, want %s", err.Error(), expected)
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewError(ErrCodeInternal, "operation failed").WithCause(cause)

	if !errors.Is(err, cause) {
		t.Error("Expected to unwrap to cause error")
	}
}

func TestAppErrorChaining(t *testing.T) {
	err := NewError(ErrCodeValidation, "validation failed").
		WithDetails("multiple errors").
		WithRequestID("req-123").
		WithTraceID("trace-456").
		WithField("email", "invalid format").
		WithField("age", "must be positive").
		WithMetadata("attempt", 1)

	if err.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want req-123", err.RequestID)
	}
	if err.TraceID != "trace-456" {
		t.Errorf("TraceID = %s, want trace-456", err.TraceID)
	}
	if len(err.Errors) != 2 {
		t.Errorf("Expected 2 field errors, got %d", len(err.Errors))
	}
	if err.Metadata["attempt"] != 1 {
		t.Error("Expected metadata 'attempt' to be 1")
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		code     ErrorCode
		status   int
	}{
		{"BadRequest", BadRequest("bad"), ErrCodeBadRequest, http.StatusBadRequest},
		{"Unauthorized", Unauthorized("unauth"), ErrCodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", Forbidden("forbidden"), ErrCodeForbidden, http.StatusForbidden},
		{"NotFound", NotFound("not found"), ErrCodeNotFound, http.StatusNotFound},
		{"Conflict", Conflict("conflict"), ErrCodeConflict, http.StatusConflict},
		{"InternalError", InternalError("internal"), ErrCodeInternal, http.StatusInternalServerError},
		{"ServiceUnavailable", ServiceUnavailable("unavailable"), ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %s, want %s", tt.err.Code, tt.code)
			}
			if tt.err.HTTPStatus != tt.status {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.status)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError("validation failed",
		FieldError{Field: "email", Message: "invalid", Code: "INVALID_FORMAT"},
		FieldError{Field: "name", Message: "required", Code: "REQUIRED"},
	)

	if err.Code != ErrCodeValidation {
		t.Errorf("Code = %s, want VALIDATION_ERROR", err.Code)
	}
	if len(err.Errors) != 2 {
		t.Errorf("Expected 2 field errors, got %d", len(err.Errors))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	err := BadRequest("invalid input").WithRequestID("req-123")

	WriteError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type application/json")
	}
}

func TestWriteErrorWithStatus(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorWithStatus(w, http.StatusTeapot, ErrCodeBadRequest, "I'm a teapot")

	if w.Code != http.StatusTeapot {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusTeapot)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected error")
	})

	wrapped := RecoverMiddleware(handler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "req-123")

	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{ServiceUnavailable("unavailable"), true},
		{NewError(ErrCodeGatewayTimeout, "timeout"), true},
		{NewError(ErrCodeBadGateway, "bad gateway"), true},
		{BadRequest("bad request"), false},
		{NotFound("not found"), false},
		{errors.New("regular error"), false},
	}

	for _, tt := range tests {
		if got := IsRetryable(tt.err); got != tt.expected {
			t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

func TestIsClientError(t *testing.T) {
	if !IsClientError(BadRequest("bad")) {
		t.Error("Expected BadRequest to be client error")
	}
	if IsClientError(InternalError("internal")) {
		t.Error("Expected InternalError to not be client error")
	}
	if IsClientError(errors.New("regular")) {
		t.Error("Expected regular error to not be client error")
	}
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(InternalError("internal")) {
		t.Error("Expected InternalError to be server error")
	}
	if IsServerError(BadRequest("bad")) {
		t.Error("Expected BadRequest to not be server error")
	}
	if IsServerError(errors.New("regular")) {
		t.Error("Expected regular error to not be server error")
	}
}

// ==================== Retry Tests ====================

func TestRetrySuccess(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return nil
	}

	err := Retry(context.Background(), nil, fn)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryEventualSuccess(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return ServiceUnavailable("temporary failure")
		}
		return nil
	}

	config := &RetryConfig{
		MaxRetries:   5,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
		RetryIf:      IsRetryable,
	}

	err := Retry(context.Background(), config, fn)
	if err != nil {
		t.Errorf("Expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryMaxRetriesExceeded(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return ServiceUnavailable("always fails")
	}

	config := &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0,
		RetryIf:      IsRetryable,
	}

	err := Retry(context.Background(), config, fn)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("Expected ErrMaxRetriesExceeded, got %v", err)
	}
	if attempts != 4 { // Initial + 3 retries
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}
}

func TestRetryNonRetryableError(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return BadRequest("not retryable")
	}

	config := &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		RetryIf:      IsRetryable,
	}

	err := Retry(context.Background(), config, fn)
	if err == nil {
		t.Error("Expected error for non-retryable failure")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fn := func() error {
		return nil
	}

	err := Retry(ctx, nil, fn)
	if !errors.Is(err, ErrContextCanceled) {
		t.Errorf("Expected ErrContextCanceled, got %v", err)
	}
}

func TestRetryWithResult(t *testing.T) {
	attempts := 0
	fn := func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, ServiceUnavailable("temp")
		}
		return 42, nil
	}

	config := &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		RetryIf:      IsRetryable,
	}

	result, err := RetryWithResult(context.Background(), config, fn)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("Expected result 42, got %d", result)
	}
}

func TestRetryWithResultMaxRetries(t *testing.T) {
	fn := func() (string, error) {
		return "", ServiceUnavailable("always fails")
	}

	config := &RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		RetryIf:      IsRetryable,
	}

	result, err := RetryWithResult(context.Background(), config, fn)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("Expected ErrMaxRetriesExceeded, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got %s", result)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay 100ms, got %v", config.InitialDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier 2.0, got %f", config.Multiplier)
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		base     time.Duration
		max      time.Duration
		mult     float64
		expected time.Duration
	}{
		{0, 100 * time.Millisecond, 10 * time.Second, 2.0, 100 * time.Millisecond},
		{1, 100 * time.Millisecond, 10 * time.Second, 2.0, 200 * time.Millisecond},
		{2, 100 * time.Millisecond, 10 * time.Second, 2.0, 400 * time.Millisecond},
		{10, 100 * time.Millisecond, 1 * time.Second, 2.0, 1 * time.Second}, // Capped at max
	}

	for _, tt := range tests {
		got := ExponentialBackoff(tt.attempt, tt.base, tt.max, tt.mult)
		if got != tt.expected {
			t.Errorf("ExponentialBackoff(%d, %v, %v, %f) = %v, want %v",
				tt.attempt, tt.base, tt.max, tt.mult, got, tt.expected)
		}
	}
}

func TestLinearBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		base     time.Duration
		max      time.Duration
		expected time.Duration
	}{
		{0, 100 * time.Millisecond, 1 * time.Second, 100 * time.Millisecond},
		{1, 100 * time.Millisecond, 1 * time.Second, 200 * time.Millisecond},
		{2, 100 * time.Millisecond, 1 * time.Second, 300 * time.Millisecond},
		{20, 100 * time.Millisecond, 1 * time.Second, 1 * time.Second}, // Capped at max
	}

	for _, tt := range tests {
		got := LinearBackoff(tt.attempt, tt.base, tt.max)
		if got != tt.expected {
			t.Errorf("LinearBackoff(%d, %v, %v) = %v, want %v",
				tt.attempt, tt.base, tt.max, got, tt.expected)
		}
	}
}

func TestConstantBackoff(t *testing.T) {
	delay := 500 * time.Millisecond
	if got := ConstantBackoff(delay); got != delay {
		t.Errorf("ConstantBackoff(%v) = %v, want %v", delay, got, delay)
	}
}

func TestRetryableFunc(t *testing.T) {
	rf := NewRetryableFunc(&RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		RetryIf:      func(err error) bool { return true },
	})

	attempts := 0
	err := rf.Do(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temp")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	attempts := 0
	err := policy.Execute(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return ServiceUnavailable("temp")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

// ==================== Timeout Tests ====================

func TestWithTimeout(t *testing.T) {
	err := WithTimeout(context.Background(), 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestWithTimeoutExceeded(t *testing.T) {
	err := WithTimeout(context.Background(), 10*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("Expected ErrTimeout, got %v", err)
	}
}

func TestWithTimeoutContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := WithTimeout(ctx, 1*time.Second, func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestWithTimeoutResult(t *testing.T) {
	result, err := WithTimeoutResult(context.Background(), 100*time.Millisecond, func(ctx context.Context) (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("Expected result 42, got %d", result)
	}
}

func TestWithTimeoutResultExceeded(t *testing.T) {
	result, err := WithTimeoutResult(context.Background(), 10*time.Millisecond, func(ctx context.Context) (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 42, nil
	})

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("Expected ErrTimeout, got %v", err)
	}
	if result != 0 {
		t.Errorf("Expected zero result, got %d", result)
	}
}

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config.ReadTimeout != 5*time.Second {
		t.Errorf("Expected ReadTimeout 5s, got %v", config.ReadTimeout)
	}
	if config.WriteTimeout != 10*time.Second {
		t.Errorf("Expected WriteTimeout 10s, got %v", config.WriteTimeout)
	}
	if config.RequestTimeout != 30*time.Second {
		t.Errorf("Expected RequestTimeout 30s, got %v", config.RequestTimeout)
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := TimeoutMiddleware(100 * time.Millisecond)
	wrapped := middleware(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTimeoutMiddlewareTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := TimeoutMiddleware(10 * time.Millisecond)
	wrapped := middleware(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status 504, got %d", w.Code)
	}
}

func TestDeadline(t *testing.T) {
	d := NewDeadline(100 * time.Millisecond)

	if d.Expired() {
		t.Error("Deadline should not be expired immediately")
	}

	remaining := d.Remaining()
	if remaining <= 0 || remaining > 100*time.Millisecond {
		t.Errorf("Unexpected remaining time: %v", remaining)
	}

	time.Sleep(150 * time.Millisecond)

	if !d.Expired() {
		t.Error("Deadline should be expired after 150ms")
	}

	if d.Remaining() != 0 {
		t.Errorf("Expected 0 remaining, got %v", d.Remaining())
	}
}

func TestDeadlineExtend(t *testing.T) {
	d := NewDeadline(50 * time.Millisecond)

	time.Sleep(30 * time.Millisecond)
	d.Extend(100 * time.Millisecond)

	time.Sleep(40 * time.Millisecond) // 70ms total

	if d.Expired() {
		t.Error("Deadline should not be expired after extension")
	}
}

func TestDeadlineReset(t *testing.T) {
	d := NewDeadline(50 * time.Millisecond)

	time.Sleep(40 * time.Millisecond)
	d.Reset()

	if d.Expired() {
		t.Error("Deadline should not be expired after reset")
	}

	remaining := d.Remaining()
	if remaining < 40*time.Millisecond {
		t.Errorf("Expected remaining > 40ms after reset, got %v", remaining)
	}
}

func TestDeadlineContext(t *testing.T) {
	d := NewDeadline(50 * time.Millisecond)
	ctx, cancel := d.Context(context.Background())
	defer cancel()

	select {
	case <-ctx.Done():
		t.Error("Context should not be done immediately")
	default:
	}

	time.Sleep(60 * time.Millisecond)

	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be done after deadline")
	}
}

func TestTimeoutHTTPClient(t *testing.T) {
	client := TimeoutHTTPClient(nil)

	if client.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", client.Timeout)
	}

	config := &TimeoutConfig{
		RequestTimeout: 5 * time.Second,
		ReadTimeout:    2 * time.Second,
		IdleTimeout:    60 * time.Second,
	}
	client = TimeoutHTTPClient(config)

	if client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", client.Timeout)
	}
}

// ==================== Bulkhead Tests ====================

func TestBulkheadExecute(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 2,
		MaxWaiting:    5,
		WaitTimeout:   100 * time.Millisecond,
	}
	bh := NewBulkhead(config)

	err := bh.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	stats := bh.Stats()
	if stats.SuccessCount != 1 {
		t.Errorf("Expected 1 success, got %d", stats.SuccessCount)
	}
}

func TestBulkheadExecuteFailure(t *testing.T) {
	bh := NewBulkhead(nil)
	expectedErr := errors.New("operation failed")

	err := bh.Execute(context.Background(), func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	stats := bh.Stats()
	if stats.FailureCount != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.FailureCount)
	}
}

func TestBulkheadConcurrentLimit(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 2,
		MaxWaiting:    0, // No waiting queue
		WaitTimeout:   10 * time.Millisecond,
	}
	bh := NewBulkhead(config)

	var running int32
	var maxConcurrent int32
	var wg sync.WaitGroup
	var rejected int32

	// Try to run 5 concurrent operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := bh.Execute(context.Background(), func() error {
				current := atomic.AddInt32(&running, 1)
				for {
					old := atomic.LoadInt32(&maxConcurrent)
					if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&running, -1)
				return nil
			})
			if err != nil {
				atomic.AddInt32(&rejected, 1)
			}
		}()
	}

	wg.Wait()

	if maxConcurrent > 2 {
		t.Errorf("Max concurrent exceeded limit: %d", maxConcurrent)
	}

	stats := bh.Stats()
	if stats.RejectedCount == 0 {
		t.Error("Expected some rejections with no waiting queue")
	}
}

func TestBulkheadWithWaitingQueue(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 1,
		MaxWaiting:    5,
		WaitTimeout:   1 * time.Second,
	}
	bh := NewBulkhead(config)

	var completed int32
	var wg sync.WaitGroup

	// Run 3 operations with 1 concurrent slot and waiting queue
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := bh.Execute(context.Background(), func() error {
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&completed, 1)
				return nil
			})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		}()
	}

	wg.Wait()

	if completed != 3 {
		t.Errorf("Expected 3 completions, got %d", completed)
	}
}

func TestBulkheadWaitingQueueFull(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 1,
		MaxWaiting:    1,
		WaitTimeout:   100 * time.Millisecond,
	}
	bh := NewBulkhead(config)

	// Fill the semaphore
	var wg sync.WaitGroup
	started := make(chan struct{})
	done := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		bh.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()

	<-started // Wait for first to start

	// Fill waiting queue
	wg.Add(1)
	go func() {
		defer wg.Done()
		bh.Execute(context.Background(), func() error {
			return nil
		})
	}()

	time.Sleep(10 * time.Millisecond) // Let it enter waiting queue

	// This should be rejected
	err := bh.TryExecute(func() error {
		return nil
	})

	if err != ErrBulkheadRejected {
		t.Errorf("Expected ErrBulkheadRejected, got %v", err)
	}

	close(done)
	wg.Wait()
}

func TestBulkheadExecuteWithResult(t *testing.T) {
	bh := NewBulkhead(nil)

	result, err := bh.ExecuteWithResult(context.Background(), func() (interface{}, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("Expected result 42, got %v", result)
	}
}

func TestBulkheadTryExecute(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 1,
		MaxWaiting:    0,
		WaitTimeout:   0,
	}
	bh := NewBulkhead(config)

	// Should succeed
	err := bh.TryExecute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestBulkheadTryExecuteRejected(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 1,
		MaxWaiting:    0,
		WaitTimeout:   0,
	}
	bh := NewBulkhead(config)

	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		bh.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()

	<-started

	err := bh.TryExecute(func() error {
		return nil
	})

	if err != ErrBulkheadRejected {
		t.Errorf("Expected ErrBulkheadRejected, got %v", err)
	}

	close(done)
}

func TestBulkheadStats(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 5,
		MaxWaiting:    10,
		WaitTimeout:   100 * time.Millisecond,
	}
	bh := NewBulkhead(config)

	// Run some successful operations
	for i := 0; i < 3; i++ {
		bh.Execute(context.Background(), func() error {
			return nil
		})
	}

	// Run some failing operations
	for i := 0; i < 2; i++ {
		bh.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	stats := bh.Stats()

	if stats.MaxConcurrent != 5 {
		t.Errorf("Expected MaxConcurrent 5, got %d", stats.MaxConcurrent)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("Expected 3 successes, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 2 {
		t.Errorf("Expected 2 failures, got %d", stats.FailureCount)
	}
	if stats.AvailableSlots != 5 {
		t.Errorf("Expected 5 available slots, got %d", stats.AvailableSlots)
	}
}

func TestSemaphoreBulkhead(t *testing.T) {
	sb := NewSemaphoreBulkhead(3, 100*time.Millisecond)

	// Acquire all slots
	for i := 0; i < 3; i++ {
		err := sb.Acquire(context.Background())
		if err != nil {
			t.Errorf("Expected to acquire slot %d, got error %v", i, err)
		}
	}

	if sb.AvailableSlots() != 0 {
		t.Errorf("Expected 0 available slots, got %d", sb.AvailableSlots())
	}

	if sb.ActiveSlots() != 3 {
		t.Errorf("Expected 3 active slots, got %d", sb.ActiveSlots())
	}

	// Release one
	sb.Release()

	if sb.AvailableSlots() != 1 {
		t.Errorf("Expected 1 available slot, got %d", sb.AvailableSlots())
	}
}

func TestSemaphoreBulkheadTimeout(t *testing.T) {
	sb := NewSemaphoreBulkhead(1, 10*time.Millisecond)

	// Acquire the only slot
	sb.Acquire(context.Background())

	// This should timeout
	err := sb.Acquire(context.Background())
	if err != ErrBulkheadTimeout {
		t.Errorf("Expected ErrBulkheadTimeout, got %v", err)
	}
}

func TestSemaphoreBulkheadTryAcquire(t *testing.T) {
	sb := NewSemaphoreBulkhead(1, 100*time.Millisecond)

	// First should succeed
	if !sb.TryAcquire() {
		t.Error("Expected first TryAcquire to succeed")
	}

	// Second should fail
	if sb.TryAcquire() {
		t.Error("Expected second TryAcquire to fail")
	}

	sb.Release()

	// Should succeed again
	if !sb.TryAcquire() {
		t.Error("Expected TryAcquire to succeed after release")
	}
}

func TestDefaultBulkheadConfig(t *testing.T) {
	config := DefaultBulkheadConfig()

	if config.MaxConcurrent != 10 {
		t.Errorf("Expected MaxConcurrent 10, got %d", config.MaxConcurrent)
	}
	if config.MaxWaiting != 100 {
		t.Errorf("Expected MaxWaiting 100, got %d", config.MaxWaiting)
	}
	if config.WaitTimeout != 30*time.Second {
		t.Errorf("Expected WaitTimeout 30s, got %v", config.WaitTimeout)
	}
}

func TestBulkheadWaitTimeout(t *testing.T) {
	config := &BulkheadConfig{
		MaxConcurrent: 1,
		MaxWaiting:    10,
		WaitTimeout:   10 * time.Millisecond,
	}
	bh := NewBulkhead(config)

	started := make(chan struct{})
	done := make(chan struct{})

	// Fill the semaphore
	go func() {
		bh.Execute(context.Background(), func() error {
			close(started)
			<-done
			return nil
		})
	}()

	<-started

	// This should timeout while waiting
	err := bh.Execute(context.Background(), func() error {
		return nil
	})

	if err != ErrBulkheadTimeout {
		t.Errorf("Expected ErrBulkheadTimeout, got %v", err)
	}

	close(done)
}
