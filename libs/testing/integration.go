package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// IntegrationSuite provides a test suite for integration tests
type IntegrationSuite struct {
	T              *testing.T
	Server         *httptest.Server
	Client         *http.Client
	MockDB         *MockDB
	MockCache      *MockCache
	MockQueue      *MockMessageQueue
	Factory        *FixtureFactory
	Env            *EnvironmentFixtures
	cleanupFuncs   []func()
	mu             sync.Mutex
	skipIntegration bool
}

// NewIntegrationSuite creates a new integration test suite
func NewIntegrationSuite(t *testing.T) *IntegrationSuite {
	suite := &IntegrationSuite{
		T:         t,
		MockDB:    NewMockDB(),
		MockCache: NewMockCache(),
		MockQueue: NewMockMessageQueue(),
		Factory:   NewFixtureFactory(),
		Env:       NewEnvironmentFixtures(),
		Client:    &http.Client{Timeout: 10 * time.Second},
	}

	// Check if we should skip integration tests
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		suite.skipIntegration = true
	}

	return suite
}

// Skip skips the current test if integration tests are disabled
func (s *IntegrationSuite) Skip() {
	if s.skipIntegration {
		s.T.Skip("Integration tests are disabled (SKIP_INTEGRATION=true)")
	}
}

// SetupServer creates a test server with the given handler
func (s *IntegrationSuite) SetupServer(handler http.Handler) {
	s.Server = httptest.NewServer(handler)
	s.RegisterCleanup(func() {
		s.Server.Close()
	})
}

// SetupTLSServer creates a test server with TLS
func (s *IntegrationSuite) SetupTLSServer(handler http.Handler) {
	s.Server = httptest.NewTLSServer(handler)
	s.Client = s.Server.Client()
	s.RegisterCleanup(func() {
		s.Server.Close()
	})
}

// RegisterCleanup registers a cleanup function to be called during teardown
func (s *IntegrationSuite) RegisterCleanup(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupFuncs = append(s.cleanupFuncs, fn)
}

// Teardown cleans up all resources
func (s *IntegrationSuite) Teardown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Call cleanup functions in reverse order
	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		s.cleanupFuncs[i]()
	}

	s.MockDB.Reset()
	s.MockCache.Reset()
	s.MockQueue.Reset()
	s.Env.Restore()
}

// Request performs an HTTP request to the test server
func (s *IntegrationSuite) Request(method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	if s.Server == nil {
		return nil, fmt.Errorf("server not initialized")
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	url := s.Server.URL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return s.Client.Do(req)
}

// GET performs a GET request
func (s *IntegrationSuite) GET(path string, headers map[string]string) (*http.Response, error) {
	return s.Request("GET", path, nil, headers)
}

// POST performs a POST request
func (s *IntegrationSuite) POST(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return s.Request("POST", path, body, headers)
}

// PUT performs a PUT request
func (s *IntegrationSuite) PUT(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return s.Request("PUT", path, body, headers)
}

// PATCH performs a PATCH request
func (s *IntegrationSuite) PATCH(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return s.Request("PATCH", path, body, headers)
}

// DELETE performs a DELETE request
func (s *IntegrationSuite) DELETE(path string, headers map[string]string) (*http.Response, error) {
	return s.Request("DELETE", path, nil, headers)
}

// ParseResponse parses a JSON response into the target
func (s *IntegrationSuite) ParseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// AssertStatus asserts the response status code
func (s *IntegrationSuite) AssertStatus(resp *http.Response, expected int) {
	s.T.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		s.T.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// TestContext creates a context with timeout for tests
func (s *IntegrationSuite) TestContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// WaitFor waits for a condition to be met
func (s *IntegrationSuite) WaitFor(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// APITest provides fluent API testing
type APITest struct {
	suite    *IntegrationSuite
	request  *http.Request
	response *http.Response
	err      error
}

// API creates a new API test
func (s *IntegrationSuite) API() *APITest {
	return &APITest{suite: s}
}

// Get sets up a GET request
func (a *APITest) Get(path string) *APITest {
	url := a.suite.Server.URL + path
	a.request, a.err = http.NewRequest("GET", url, nil)
	return a
}

// Post sets up a POST request
func (a *APITest) Post(path string) *APITest {
	url := a.suite.Server.URL + path
	a.request, a.err = http.NewRequest("POST", url, nil)
	return a
}

// Put sets up a PUT request
func (a *APITest) Put(path string) *APITest {
	url := a.suite.Server.URL + path
	a.request, a.err = http.NewRequest("PUT", url, nil)
	return a
}

// Delete sets up a DELETE request
func (a *APITest) Delete(path string) *APITest {
	url := a.suite.Server.URL + path
	a.request, a.err = http.NewRequest("DELETE", url, nil)
	return a
}

// WithHeader adds a header to the request
func (a *APITest) WithHeader(key, value string) *APITest {
	if a.request != nil {
		a.request.Header.Set(key, value)
	}
	return a
}

// WithJSON sets the request body as JSON
func (a *APITest) WithJSON(body interface{}) *APITest {
	if a.request == nil {
		return a
	}
	data, err := json.Marshal(body)
	if err != nil {
		a.err = err
		return a
	}
	a.request.Body = io.NopCloser(bytes.NewReader(data))
	a.request.Header.Set("Content-Type", "application/json")
	return a
}

// WithAuth adds an authorization header
func (a *APITest) WithAuth(token string) *APITest {
	return a.WithHeader("Authorization", "Bearer "+token)
}

// WithAPIKey adds an API key header
func (a *APITest) WithAPIKey(key string) *APITest {
	return a.WithHeader("X-API-Key", key)
}

// WithRequestID adds a request ID header
func (a *APITest) WithRequestID(id string) *APITest {
	return a.WithHeader("X-Request-ID", id)
}

// Expect executes the request and returns an expectation chain
func (a *APITest) Expect() *APIExpectation {
	if a.err != nil {
		a.suite.T.Fatalf("Request setup failed: %v", a.err)
	}
	a.response, a.err = a.suite.Client.Do(a.request)
	if a.err != nil {
		a.suite.T.Fatalf("Request failed: %v", a.err)
	}
	return &APIExpectation{test: a}
}

// APIExpectation provides response assertions
type APIExpectation struct {
	test *APITest
}

// Status asserts the response status code
func (e *APIExpectation) Status(expected int) *APIExpectation {
	e.test.suite.T.Helper()
	if e.test.response.StatusCode != expected {
		body, _ := io.ReadAll(e.test.response.Body)
		e.test.suite.T.Errorf("Expected status %d, got %d. Body: %s",
			expected, e.test.response.StatusCode, string(body))
	}
	return e
}

// Header asserts a response header
func (e *APIExpectation) Header(key, expected string) *APIExpectation {
	e.test.suite.T.Helper()
	actual := e.test.response.Header.Get(key)
	if actual != expected {
		e.test.suite.T.Errorf("Expected header %s=%q, got %q", key, expected, actual)
	}
	return e
}

// HeaderContains asserts a response header contains a substring
func (e *APIExpectation) HeaderContains(key, substr string) *APIExpectation {
	e.test.suite.T.Helper()
	actual := e.test.response.Header.Get(key)
	if !strings.Contains(actual, substr) {
		e.test.suite.T.Errorf("Expected header %s to contain %q, got %q", key, substr, actual)
	}
	return e
}

// BodyContains asserts the body contains a substring
func (e *APIExpectation) BodyContains(substr string) *APIExpectation {
	e.test.suite.T.Helper()
	body, err := io.ReadAll(e.test.response.Body)
	if err != nil {
		e.test.suite.T.Fatalf("Failed to read body: %v", err)
	}
	e.test.response.Body = io.NopCloser(bytes.NewReader(body))
	if !strings.Contains(string(body), substr) {
		e.test.suite.T.Errorf("Expected body to contain %q, got %s", substr, string(body))
	}
	return e
}

// JSON parses the response body as JSON into target
func (e *APIExpectation) JSON(target interface{}) *APIExpectation {
	e.test.suite.T.Helper()
	if err := json.NewDecoder(e.test.response.Body).Decode(target); err != nil {
		e.test.suite.T.Fatalf("Failed to parse JSON: %v", err)
	}
	return e
}

// End closes the response body
func (e *APIExpectation) End() {
	if e.test.response != nil && e.test.response.Body != nil {
		e.test.response.Body.Close()
	}
}

// Benchmark provides benchmarking utilities
type Benchmark struct {
	b        *testing.B
	name     string
	setup    func()
	teardown func()
}

// NewBenchmark creates a new benchmark helper
func NewBenchmark(b *testing.B, name string) *Benchmark {
	return &Benchmark{b: b, name: name}
}

// Setup sets the setup function
func (bench *Benchmark) Setup(fn func()) *Benchmark {
	bench.setup = fn
	return bench
}

// Teardown sets the teardown function
func (bench *Benchmark) Teardown(fn func()) *Benchmark {
	bench.teardown = fn
	return bench
}

// Run runs the benchmark
func (bench *Benchmark) Run(fn func()) {
	if bench.setup != nil {
		bench.setup()
	}

	bench.b.ResetTimer()
	for i := 0; i < bench.b.N; i++ {
		fn()
	}
	bench.b.StopTimer()

	if bench.teardown != nil {
		bench.teardown()
	}
}

// RunParallel runs the benchmark in parallel
func (bench *Benchmark) RunParallel(fn func()) {
	if bench.setup != nil {
		bench.setup()
	}

	bench.b.ResetTimer()
	bench.b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fn()
		}
	})
	bench.b.StopTimer()

	if bench.teardown != nil {
		bench.teardown()
	}
}

// SubTest represents a parameterized subtest
type SubTest[T any] struct {
	Name string
	Data T
}

// RunSubTests runs parameterized subtests
func RunSubTests[T any](t *testing.T, tests []SubTest[T], fn func(t *testing.T, data T)) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			fn(t, tt.Data)
		})
	}
}

// TableTest represents a table-driven test case
type TableTest struct {
	Name     string
	Setup    func()
	Run      func(t *testing.T)
	Teardown func()
	Skip     bool
}

// RunTableTests runs table-driven tests
func RunTableTests(t *testing.T, tests []TableTest) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Skip {
				t.Skip()
			}
			if tt.Setup != nil {
				tt.Setup()
			}
			defer func() {
				if tt.Teardown != nil {
					tt.Teardown()
				}
			}()
			tt.Run(t)
		})
	}
}
