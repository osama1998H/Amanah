package testing

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// MockDB implements a mock database for testing
type MockDB struct {
	mu           sync.RWMutex
	data         map[string]map[string]interface{} // table -> id -> row
	queries      []QueryRecord
	shouldFail   bool
	failureError error
	latency      time.Duration
}

// QueryRecord records a database query for verification
type QueryRecord struct {
	Query     string
	Args      []interface{}
	Timestamp time.Time
}

// NewMockDB creates a new mock database
func NewMockDB() *MockDB {
	return &MockDB{
		data:    make(map[string]map[string]interface{}),
		queries: make([]QueryRecord, 0),
	}
}

// SetFailure configures the mock to fail with the given error
func (m *MockDB) SetFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = true
	m.failureError = err
}

// ClearFailure removes the failure configuration
func (m *MockDB) ClearFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = false
	m.failureError = nil
}

// SetLatency adds artificial latency to operations
func (m *MockDB) SetLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = d
}

// Insert inserts a record into the mock database
func (m *MockDB) Insert(table string, id string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return m.failureError
	}

	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	if _, ok := m.data[table]; !ok {
		m.data[table] = make(map[string]interface{})
	}
	m.data[table][id] = data

	m.queries = append(m.queries, QueryRecord{
		Query:     "INSERT INTO " + table,
		Args:      []interface{}{id, data},
		Timestamp: time.Now(),
	})

	return nil
}

// Get retrieves a record from the mock database
func (m *MockDB) Get(table string, id string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	m.queries = append(m.queries, QueryRecord{
		Query:     "SELECT FROM " + table,
		Args:      []interface{}{id},
		Timestamp: time.Now(),
	})

	if tableData, ok := m.data[table]; ok {
		if row, ok := tableData[id]; ok {
			return row, true
		}
	}
	return nil, false
}

// Delete removes a record from the mock database
func (m *MockDB) Delete(table string, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return m.failureError
	}

	if tableData, ok := m.data[table]; ok {
		delete(tableData, id)
	}

	m.queries = append(m.queries, QueryRecord{
		Query:     "DELETE FROM " + table,
		Args:      []interface{}{id},
		Timestamp: time.Now(),
	})

	return nil
}

// GetQueries returns all recorded queries
func (m *MockDB) GetQueries() []QueryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]QueryRecord{}, m.queries...)
}

// ClearQueries clears the recorded queries
func (m *MockDB) ClearQueries() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = make([]QueryRecord, 0)
}

// Reset clears all data and queries
func (m *MockDB) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]map[string]interface{})
	m.queries = make([]QueryRecord, 0)
	m.shouldFail = false
	m.failureError = nil
	m.latency = 0
}

// MockCache implements a mock cache for testing
type MockCache struct {
	mu           sync.RWMutex
	data         map[string]cacheEntry
	shouldFail   bool
	failureError error
	hits         int
	misses       int
}

type cacheEntry struct {
	value      interface{}
	expiration time.Time
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]cacheEntry),
	}
}

// Set stores a value in the cache
func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return m.failureError
	}

	expiration := time.Time{}
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	m.data[key] = cacheEntry{
		value:      value,
		expiration: expiration,
	}
	return nil
}

// Get retrieves a value from the cache
func (m *MockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.data[key]
	if !ok {
		m.misses++
		return nil, false
	}

	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		delete(m.data, key)
		m.misses++
		return nil, false
	}

	m.hits++
	return entry.value, true
}

// Delete removes a value from the cache
func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return m.failureError
	}

	delete(m.data, key)
	return nil
}

// SetFailure configures the mock to fail
func (m *MockCache) SetFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = true
	m.failureError = err
}

// ClearFailure removes the failure configuration
func (m *MockCache) ClearFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = false
	m.failureError = nil
}

// Stats returns cache statistics
func (m *MockCache) Stats() (hits, misses int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hits, m.misses
}

// Reset clears all data and stats
func (m *MockCache) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]cacheEntry)
	m.hits = 0
	m.misses = 0
	m.shouldFail = false
	m.failureError = nil
}

// MockHTTPClient implements a mock HTTP client for testing
type MockHTTPClient struct {
	mu         sync.Mutex
	responses  map[string]*http.Response
	requests   []*http.Request
	defaultErr error
	latency    time.Duration
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*http.Response),
		requests:  make([]*http.Request, 0),
	}
}

// SetResponse sets a mock response for a given URL
func (m *MockHTTPClient) SetResponse(url string, statusCode int, body string, headers map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(stringReader(body)),
		Header:     make(http.Header),
	}

	for k, v := range headers {
		resp.Header.Set(k, v)
	}

	m.responses[url] = resp
}

// SetJSONResponse sets a JSON response for a given URL
func (m *MockHTTPClient) SetJSONResponse(url string, statusCode int, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m.SetResponse(url, statusCode, string(body), map[string]string{
		"Content-Type": "application/json",
	})
	return nil
}

// SetError sets a default error for all requests
func (m *MockHTTPClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultErr = err
}

// SetLatency adds artificial latency
func (m *MockHTTPClient) SetLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = d
}

// Do executes a mock HTTP request
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, req)

	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	if m.defaultErr != nil {
		return nil, m.defaultErr
	}

	if resp, ok := m.responses[req.URL.String()]; ok {
		return resp, nil
	}

	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(stringReader(`{"error": "not found"}`)),
	}, nil
}

// GetRequests returns all recorded requests
func (m *MockHTTPClient) GetRequests() []*http.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*http.Request{}, m.requests...)
}

// Reset clears all responses and requests
func (m *MockHTTPClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = make(map[string]*http.Response)
	m.requests = make([]*http.Request, 0)
	m.defaultErr = nil
	m.latency = 0
}

// stringReader creates an io.Reader from a string
type stringReader string

func (s stringReader) Read(p []byte) (n int, err error) {
	n = copy(p, s)
	return n, io.EOF
}

// MockMessageQueue implements a mock message queue for testing
type MockMessageQueue struct {
	mu           sync.Mutex
	messages     map[string][]Message
	subscribers  map[string][]chan Message
	shouldFail   bool
	failureError error
}

// Message represents a queue message
type Message struct {
	ID        string
	Topic     string
	Body      []byte
	Headers   map[string]string
	Timestamp time.Time
}

// NewMockMessageQueue creates a new mock message queue
func NewMockMessageQueue() *MockMessageQueue {
	return &MockMessageQueue{
		messages:    make(map[string][]Message),
		subscribers: make(map[string][]chan Message),
	}
}

// Publish publishes a message to a topic
func (m *MockMessageQueue) Publish(topic string, body []byte, headers map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return m.failureError
	}

	msg := Message{
		ID:        generateID(),
		Topic:     topic,
		Body:      body,
		Headers:   headers,
		Timestamp: time.Now(),
	}

	m.messages[topic] = append(m.messages[topic], msg)

	// Notify subscribers
	if subs, ok := m.subscribers[topic]; ok {
		for _, ch := range subs {
			select {
			case ch <- msg:
			default:
			}
		}
	}

	return nil
}

// Subscribe subscribes to a topic
func (m *MockMessageQueue) Subscribe(topic string) <-chan Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Message, 100)
	m.subscribers[topic] = append(m.subscribers[topic], ch)
	return ch
}

// GetMessages returns all messages for a topic
func (m *MockMessageQueue) GetMessages(topic string) []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Message{}, m.messages[topic]...)
}

// SetFailure configures the mock to fail
func (m *MockMessageQueue) SetFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = true
	m.failureError = err
}

// ClearFailure removes the failure configuration
func (m *MockMessageQueue) ClearFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = false
	m.failureError = nil
}

// Reset clears all messages and subscribers
func (m *MockMessageQueue) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make(map[string][]Message)
	for _, subs := range m.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	m.subscribers = make(map[string][]chan Message)
	m.shouldFail = false
	m.failureError = nil
}

// MockTx implements a mock database transaction
type MockTx struct {
	db         *MockDB
	committed  bool
	rolledback bool
}

// NewMockTx creates a mock transaction
func (m *MockDB) BeginTx() *MockTx {
	return &MockTx{db: m}
}

// Commit commits the transaction
func (tx *MockTx) Commit() error {
	tx.committed = true
	return nil
}

// Rollback rolls back the transaction
func (tx *MockTx) Rollback() error {
	tx.rolledback = true
	return nil
}

// IsCommitted returns true if committed
func (tx *MockTx) IsCommitted() bool {
	return tx.committed
}

// IsRolledback returns true if rolled back
func (tx *MockTx) IsRolledback() bool {
	return tx.rolledback
}

// Helper function to generate IDs
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}

// MockRows implements sql.Rows for testing
type MockRows struct {
	columns []string
	data    [][]interface{}
	index   int
	closed  bool
}

// NewMockRows creates mock rows
func NewMockRows(columns []string, data [][]interface{}) *MockRows {
	return &MockRows{
		columns: columns,
		data:    data,
		index:   -1,
	}
}

// Columns returns column names
func (r *MockRows) Columns() ([]string, error) {
	return r.columns, nil
}

// Next advances to the next row
func (r *MockRows) Next() bool {
	r.index++
	return r.index < len(r.data)
}

// Scan scans the current row
func (r *MockRows) Scan(dest ...interface{}) error {
	if r.index < 0 || r.index >= len(r.data) {
		return sql.ErrNoRows
	}
	row := r.data[r.index]
	for i, v := range dest {
		if i < len(row) {
			switch d := v.(type) {
			case *string:
				if s, ok := row[i].(string); ok {
					*d = s
				}
			case *int:
				if n, ok := row[i].(int); ok {
					*d = n
				}
			case *int64:
				if n, ok := row[i].(int64); ok {
					*d = n
				}
			case *float64:
				if f, ok := row[i].(float64); ok {
					*d = f
				}
			case *bool:
				if b, ok := row[i].(bool); ok {
					*d = b
				}
			case *time.Time:
				if t, ok := row[i].(time.Time); ok {
					*d = t
				}
			case *interface{}:
				*d = row[i]
			}
		}
	}
	return nil
}

// Close closes the rows
func (r *MockRows) Close() error {
	r.closed = true
	return nil
}

// Err returns any error
func (r *MockRows) Err() error {
	return nil
}

// MockServer creates a test HTTP server with custom handlers
type MockServer struct {
	Server   *httptest.Server
	handlers map[string]http.HandlerFunc
	mu       sync.Mutex
	requests []*http.Request
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	ms := &MockServer{
		handlers: make(map[string]http.HandlerFunc),
		requests: make([]*http.Request, 0),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.mu.Lock()
		ms.requests = append(ms.requests, r)
		handler, ok := ms.handlers[r.URL.Path]
		ms.mu.Unlock()

		if ok {
			handler(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	return ms
}

// Handle registers a handler for a path
func (ms *MockServer) Handle(path string, handler http.HandlerFunc) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.handlers[path] = handler
}

// HandleJSON registers a JSON handler
func (ms *MockServer) HandleJSON(path string, statusCode int, data interface{}) {
	ms.Handle(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(data)
	})
}

// GetRequests returns recorded requests
func (ms *MockServer) GetRequests() []*http.Request {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return append([]*http.Request{}, ms.requests...)
}

// URL returns the server URL
func (ms *MockServer) URL() string {
	return ms.Server.URL
}

// Close stops the server
func (ms *MockServer) Close() {
	ms.Server.Close()
}

// Reset clears handlers and requests
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.handlers = make(map[string]http.HandlerFunc)
	ms.requests = make([]*http.Request, 0)
}
