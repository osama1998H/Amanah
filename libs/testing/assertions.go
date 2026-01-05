package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// T is an interface that matches *testing.T
type T interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	FailNow()
}

// Assertions provides assertion methods for testing
type Assertions struct {
	t T
}

// NewAssertions creates a new assertions helper
func NewAssertions(t T) *Assertions {
	return &Assertions{t: t}
}

// Equal asserts that two values are equal
func (a *Assertions) Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		a.t.Errorf("Expected %v (type %T), got %v (type %T)%s",
			expected, expected, actual, actual, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// NotEqual asserts that two values are not equal
func (a *Assertions) NotEqual(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if reflect.DeepEqual(expected, actual) {
		a.t.Errorf("Expected values to be different, got %v%s",
			actual, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Nil asserts that a value is nil
func (a *Assertions) Nil(object interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if !isNil(object) {
		a.t.Errorf("Expected nil, got %v%s", object, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// NotNil asserts that a value is not nil
func (a *Assertions) NotNil(object interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if isNil(object) {
		a.t.Errorf("Expected non-nil value%s", formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// True asserts that a value is true
func (a *Assertions) True(value bool, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if !value {
		a.t.Errorf("Expected true%s", formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// False asserts that a value is false
func (a *Assertions) False(value bool, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if value {
		a.t.Errorf("Expected false%s", formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// NoError asserts that an error is nil
func (a *Assertions) NoError(err error, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if err != nil {
		a.t.Errorf("Unexpected error: %v%s", err, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Error asserts that an error is not nil
func (a *Assertions) Error(err error, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("Expected error but got nil%s", formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// ErrorIs asserts that an error matches a target error
func (a *Assertions) ErrorIs(err, target error, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("Expected error %v but got nil%s", target, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	if !strings.Contains(err.Error(), target.Error()) {
		a.t.Errorf("Expected error containing %v, got %v%s", target, err, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// ErrorContains asserts that an error message contains a substring
func (a *Assertions) ErrorContains(err error, substring string, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("Expected error containing %q but got nil%s", substring, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	if !strings.Contains(err.Error(), substring) {
		a.t.Errorf("Expected error containing %q, got %q%s", substring, err.Error(), formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Contains asserts that a string contains a substring
func (a *Assertions) Contains(s, substr string, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if !strings.Contains(s, substr) {
		a.t.Errorf("Expected %q to contain %q%s", s, substr, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// NotContains asserts that a string does not contain a substring
func (a *Assertions) NotContains(s, substr string, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	if strings.Contains(s, substr) {
		a.t.Errorf("Expected %q to not contain %q%s", s, substr, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Matches asserts that a string matches a regex pattern
func (a *Assertions) Matches(pattern, s string, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	matched, err := regexp.MatchString(pattern, s)
	if err != nil {
		a.t.Errorf("Invalid regex pattern: %s%s", pattern, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	if !matched {
		a.t.Errorf("Expected %q to match pattern %q%s", s, pattern, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Len asserts that a collection has the expected length
func (a *Assertions) Len(object interface{}, length int, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	objLen := reflect.ValueOf(object).Len()
	if objLen != length {
		a.t.Errorf("Expected length %d, got %d%s", length, objLen, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Empty asserts that a collection is empty
func (a *Assertions) Empty(object interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	objLen := reflect.ValueOf(object).Len()
	if objLen != 0 {
		a.t.Errorf("Expected empty collection, got length %d%s", objLen, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// NotEmpty asserts that a collection is not empty
func (a *Assertions) NotEmpty(object interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	objLen := reflect.ValueOf(object).Len()
	if objLen == 0 {
		a.t.Errorf("Expected non-empty collection%s", formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// InDelta asserts that two floats are within delta of each other
func (a *Assertions) InDelta(expected, actual, delta float64, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	diff := expected - actual
	if diff < -delta || diff > delta {
		a.t.Errorf("Expected %f to be within %f of %f%s", actual, delta, expected, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Greater asserts that first is greater than second
func (a *Assertions) Greater(first, second interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	result := compareValues(first, second)
	if result != 1 {
		a.t.Errorf("Expected %v > %v%s", first, second, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// GreaterOrEqual asserts that first is greater than or equal to second
func (a *Assertions) GreaterOrEqual(first, second interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	result := compareValues(first, second)
	if result < 0 {
		a.t.Errorf("Expected %v >= %v%s", first, second, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Less asserts that first is less than second
func (a *Assertions) Less(first, second interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	result := compareValues(first, second)
	if result != -1 {
		a.t.Errorf("Expected %v < %v%s", first, second, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// LessOrEqual asserts that first is less than or equal to second
func (a *Assertions) LessOrEqual(first, second interface{}, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	result := compareValues(first, second)
	if result > 0 {
		a.t.Errorf("Expected %v <= %v%s", first, second, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Panics asserts that a function panics
func (a *Assertions) Panics(fn func(), msgAndArgs ...interface{}) bool {
	a.t.Helper()
	defer func() {
		if r := recover(); r == nil {
			a.t.Errorf("Expected panic%s", formatMsgAndArgs(msgAndArgs...))
		}
	}()
	fn()
	return true
}

// NotPanics asserts that a function does not panic
func (a *Assertions) NotPanics(fn func(), msgAndArgs ...interface{}) bool {
	a.t.Helper()
	defer func() {
		if r := recover(); r != nil {
			a.t.Errorf("Unexpected panic: %v%s", r, formatMsgAndArgs(msgAndArgs...))
		}
	}()
	fn()
	return true
}

// Eventually asserts that a condition is met within a timeout
func (a *Assertions) Eventually(condition func() bool, timeout, interval time.Duration, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	a.t.Errorf("Condition not met within %s%s", timeout, formatMsgAndArgs(msgAndArgs...))
	return false
}

// Never asserts that a condition is never met within a timeout
func (a *Assertions) Never(condition func() bool, timeout, interval time.Duration, msgAndArgs ...interface{}) bool {
	a.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			a.t.Errorf("Condition was met when it shouldn't be%s", formatMsgAndArgs(msgAndArgs...))
			return false
		}
		time.Sleep(interval)
	}
	return true
}

// HTTPAssertions provides HTTP-specific assertions
type HTTPAssertions struct {
	*Assertions
}

// NewHTTPAssertions creates HTTP assertions
func NewHTTPAssertions(t T) *HTTPAssertions {
	return &HTTPAssertions{Assertions: NewAssertions(t)}
}

// StatusCode asserts the response status code
func (h *HTTPAssertions) StatusCode(resp *http.Response, expected int, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	if resp.StatusCode != expected {
		h.t.Errorf("Expected status %d, got %d%s", expected, resp.StatusCode, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// StatusCodeRecorder asserts the response recorder status code
func (h *HTTPAssertions) StatusCodeRecorder(w *httptest.ResponseRecorder, expected int, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	if w.Code != expected {
		h.t.Errorf("Expected status %d, got %d%s", expected, w.Code, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Header asserts that a response has a specific header value
func (h *HTTPAssertions) Header(resp *http.Response, key, expected string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	actual := resp.Header.Get(key)
	if actual != expected {
		h.t.Errorf("Expected header %s=%q, got %q%s", key, expected, actual, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// HeaderRecorder asserts that a response recorder has a specific header value
func (h *HTTPAssertions) HeaderRecorder(w *httptest.ResponseRecorder, key, expected string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	actual := w.Header().Get(key)
	if actual != expected {
		h.t.Errorf("Expected header %s=%q, got %q%s", key, expected, actual, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// HeaderExists asserts that a response has a header
func (h *HTTPAssertions) HeaderExists(resp *http.Response, key string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	if resp.Header.Get(key) == "" {
		h.t.Errorf("Expected header %s to exist%s", key, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// ContentType asserts the Content-Type header
func (h *HTTPAssertions) ContentType(resp *http.Response, expected string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, expected) {
		h.t.Errorf("Expected Content-Type containing %q, got %q%s", expected, ct, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// BodyContains asserts that the response body contains a string
func (h *HTTPAssertions) BodyContains(resp *http.Response, substr string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Errorf("Failed to read response body: %v", err)
		return false
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body
	if !strings.Contains(string(body), substr) {
		h.t.Errorf("Expected body to contain %q%s", substr, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// BodyContainsRecorder asserts that the response recorder body contains a string
func (h *HTTPAssertions) BodyContainsRecorder(w *httptest.ResponseRecorder, substr string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	if !strings.Contains(w.Body.String(), substr) {
		h.t.Errorf("Expected body to contain %q, got %q%s", substr, w.Body.String(), formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// JSONResponse asserts and parses a JSON response
func (h *HTTPAssertions) JSONResponse(w *httptest.ResponseRecorder, target interface{}, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	if err := json.NewDecoder(w.Body).Decode(target); err != nil {
		h.t.Errorf("Failed to parse JSON response: %v%s", err, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// JSONEqual asserts that two JSON objects are equal
func (h *HTTPAssertions) JSONEqual(expected, actual string, msgAndArgs ...interface{}) bool {
	h.t.Helper()
	var exp, act interface{}
	if err := json.Unmarshal([]byte(expected), &exp); err != nil {
		h.t.Errorf("Failed to parse expected JSON: %v", err)
		return false
	}
	if err := json.Unmarshal([]byte(actual), &act); err != nil {
		h.t.Errorf("Failed to parse actual JSON: %v", err)
		return false
	}
	if !reflect.DeepEqual(exp, act) {
		h.t.Errorf("JSON not equal.\nExpected: %s\nActual: %s%s", expected, actual, formatMsgAndArgs(msgAndArgs...))
		return false
	}
	return true
}

// Helper functions

func formatMsgAndArgs(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		return ": " + fmt.Sprint(msgAndArgs[0])
	}
	return ": " + fmt.Sprintf(fmt.Sprint(msgAndArgs[0]), msgAndArgs[1:]...)
}

func isNil(object interface{}) bool {
	if object == nil {
		return true
	}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}
	return false
}

func compareValues(first, second interface{}) int {
	// Returns -1 if first < second, 0 if equal, 1 if first > second
	v1 := reflect.ValueOf(first)
	v2 := reflect.ValueOf(second)

	switch v1.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i1, i2 := v1.Int(), v2.Int()
		if i1 < i2 {
			return -1
		} else if i1 > i2 {
			return 1
		}
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u1, u2 := v1.Uint(), v2.Uint()
		if u1 < u2 {
			return -1
		} else if u1 > u2 {
			return 1
		}
		return 0
	case reflect.Float32, reflect.Float64:
		f1, f2 := v1.Float(), v2.Float()
		if f1 < f2 {
			return -1
		} else if f1 > f2 {
			return 1
		}
		return 0
	case reflect.String:
		s1, s2 := v1.String(), v2.String()
		if s1 < s2 {
			return -1
		} else if s1 > s2 {
			return 1
		}
		return 0
	}
	return 0
}

// Require wraps Assertions and fails immediately on assertion failure
type Require struct {
	*Assertions
}

// NewRequire creates a new require assertions helper
func NewRequire(t T) *Require {
	return &Require{Assertions: NewAssertions(t)}
}

// Equal asserts equality and fails immediately if not
func (r *Require) Equal(expected, actual interface{}, msgAndArgs ...interface{}) {
	r.t.Helper()
	if !r.Assertions.Equal(expected, actual, msgAndArgs...) {
		r.t.FailNow()
	}
}

// NoError asserts no error and fails immediately if there is one
func (r *Require) NoError(err error, msgAndArgs ...interface{}) {
	r.t.Helper()
	if !r.Assertions.NoError(err, msgAndArgs...) {
		r.t.FailNow()
	}
}

// NotNil asserts non-nil and fails immediately if nil
func (r *Require) NotNil(object interface{}, msgAndArgs ...interface{}) {
	r.t.Helper()
	if !r.Assertions.NotNil(object, msgAndArgs...) {
		r.t.FailNow()
	}
}

// True asserts true and fails immediately if false
func (r *Require) True(value bool, msgAndArgs ...interface{}) {
	r.t.Helper()
	if !r.Assertions.True(value, msgAndArgs...) {
		r.t.FailNow()
	}
}
