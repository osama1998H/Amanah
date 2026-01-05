package resilience

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorCode represents a standardized error code
type ErrorCode string

// Standard error codes
const (
	// Client errors (4xx)
	ErrCodeBadRequest       ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	ErrCodeConflict         ErrorCode = "CONFLICT"
	ErrCodeGone             ErrorCode = "GONE"
	ErrCodeUnprocessable    ErrorCode = "UNPROCESSABLE_ENTITY"
	ErrCodeTooManyRequests  ErrorCode = "TOO_MANY_REQUESTS"

	// Server errors (5xx)
	ErrCodeInternal         ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotImplemented   ErrorCode = "NOT_IMPLEMENTED"
	ErrCodeBadGateway       ErrorCode = "BAD_GATEWAY"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeGatewayTimeout   ErrorCode = "GATEWAY_TIMEOUT"

	// Business errors
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
	ErrCodeInsufficientFunds ErrorCode = "INSUFFICIENT_FUNDS"
	ErrCodeDuplicateEntry   ErrorCode = "DUPLICATE_ENTRY"
	ErrCodeExpired          ErrorCode = "EXPIRED"
	ErrCodeInvalidState     ErrorCode = "INVALID_STATE"
	ErrCodeDependencyFailed ErrorCode = "DEPENDENCY_FAILED"
)

// HTTPStatus returns the HTTP status code for the error code
func (c ErrorCode) HTTPStatus() int {
	switch c {
	case ErrCodeBadRequest, ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case ErrCodeConflict, ErrCodeDuplicateEntry:
		return http.StatusConflict
	case ErrCodeGone, ErrCodeExpired:
		return http.StatusGone
	case ErrCodeUnprocessable, ErrCodeInsufficientFunds, ErrCodeInvalidState:
		return http.StatusUnprocessableEntity
	case ErrCodeTooManyRequests:
		return http.StatusTooManyRequests
	case ErrCodeInternal:
		return http.StatusInternalServerError
	case ErrCodeNotImplemented:
		return http.StatusNotImplemented
	case ErrCodeBadGateway, ErrCodeDependencyFailed:
		return http.StatusBadGateway
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeGatewayTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// AppError represents a standardized application error
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	Errors     []FieldError           `json:"errors,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	HTTPStatus int                    `json:"-"`
	Cause      error                  `json:"-"`
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s - %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the cause of the error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithCause adds the underlying cause
func (e *AppError) WithCause(err error) *AppError {
	e.Cause = err
	return e
}

// WithRequestID adds a request ID
func (e *AppError) WithRequestID(id string) *AppError {
	e.RequestID = id
	return e
}

// WithTraceID adds a trace ID
func (e *AppError) WithTraceID(id string) *AppError {
	e.TraceID = id
	return e
}

// WithField adds a field error
func (e *AppError) WithField(field, message string) *AppError {
	e.Errors = append(e.Errors, FieldError{
		Field:   field,
		Message: message,
	})
	return e
}

// WithMetadata adds metadata
func (e *AppError) WithMetadata(key string, value interface{}) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// NewError creates a new application error
func NewError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: code.HTTPStatus(),
	}
}

// Common error constructors

// BadRequest creates a bad request error
func BadRequest(message string) *AppError {
	return NewError(ErrCodeBadRequest, message)
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return NewError(ErrCodeUnauthorized, message)
}

// Forbidden creates a forbidden error
func Forbidden(message string) *AppError {
	return NewError(ErrCodeForbidden, message)
}

// NotFound creates a not found error
func NotFound(message string) *AppError {
	return NewError(ErrCodeNotFound, message)
}

// Conflict creates a conflict error
func Conflict(message string) *AppError {
	return NewError(ErrCodeConflict, message)
}

// InternalError creates an internal server error
func InternalError(message string) *AppError {
	return NewError(ErrCodeInternal, message)
}

// ServiceUnavailable creates a service unavailable error
func ServiceUnavailable(message string) *AppError {
	return NewError(ErrCodeServiceUnavailable, message)
}

// ValidationError creates a validation error with field errors
func ValidationError(message string, errors ...FieldError) *AppError {
	e := NewError(ErrCodeValidation, message)
	e.Errors = errors
	return e
}

// ErrorResponse represents the JSON error response
type ErrorResponse struct {
	Success   bool                   `json:"success"`
	Error     *AppError              `json:"error"`
}

// WriteError writes an error response to the HTTP response writer
func WriteError(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	resp := ErrorResponse{
		Success: false,
		Error:   err,
	}

	json.NewEncoder(w).Encode(resp)
}

// WriteErrorWithStatus writes an error with a custom status code
func WriteErrorWithStatus(w http.ResponseWriter, status int, code ErrorCode, message string) {
	err := NewError(code, message)
	err.HTTPStatus = status
	WriteError(w, err)
}

// RecoverMiddleware recovers from panics and returns a proper error response
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				err := InternalError("An unexpected error occurred")
				if e, ok := rec.(error); ok {
					err = err.WithCause(e)
				}
				err = err.WithRequestID(r.Header.Get("X-Request-ID"))
				WriteError(w, err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeServiceUnavailable, ErrCodeGatewayTimeout, ErrCodeBadGateway:
			return true
		}
	}
	return false
}

// IsClientError checks if an error is a client error (4xx)
func IsClientError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.HTTPStatus >= 400 && appErr.HTTPStatus < 500
	}
	return false
}

// IsServerError checks if an error is a server error (5xx)
func IsServerError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.HTTPStatus >= 500
	}
	return false
}
