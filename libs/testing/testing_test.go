package testing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ==================== Mock Tests ====================

func TestMockDB(t *testing.T) {
	db := NewMockDB()

	// Test Insert and Get
	err := db.Insert("users", "1", map[string]string{"name": "John"})
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}

	data, found := db.Get("users", "1")
	if !found {
		t.Error("Expected to find inserted data")
	}
	if data.(map[string]string)["name"] != "John" {
		t.Error("Expected name to be John")
	}

	// Test Get non-existent
	_, found = db.Get("users", "999")
	if found {
		t.Error("Should not find non-existent data")
	}

	// Test Delete
	err = db.Delete("users", "1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, found = db.Get("users", "1")
	if found {
		t.Error("Should not find deleted data")
	}
}

func TestMockDBFailure(t *testing.T) {
	db := NewMockDB()
	expectedErr := errors.New("database error")
	db.SetFailure(expectedErr)

	err := db.Insert("users", "1", "data")
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	db.ClearFailure()
	err = db.Insert("users", "1", "data")
	if err != nil {
		t.Errorf("Expected no error after clear, got %v", err)
	}
}

func TestMockDBLatency(t *testing.T) {
	db := NewMockDB()
	db.SetLatency(50 * time.Millisecond)

	start := time.Now()
	db.Insert("users", "1", "data")
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond {
		t.Errorf("Expected at least 50ms latency, got %v", elapsed)
	}
}

func TestMockDBQueries(t *testing.T) {
	db := NewMockDB()
	db.Insert("users", "1", "data")
	db.Get("users", "1")
	db.Delete("users", "1")

	queries := db.GetQueries()
	if len(queries) != 3 {
		t.Errorf("Expected 3 queries, got %d", len(queries))
	}

	db.ClearQueries()
	if len(db.GetQueries()) != 0 {
		t.Error("Expected 0 queries after clear")
	}
}

func TestMockDBReset(t *testing.T) {
	db := NewMockDB()
	db.Insert("users", "1", "data")
	db.SetFailure(errors.New("error"))
	db.SetLatency(100 * time.Millisecond)

	db.Reset()

	if db.shouldFail {
		t.Error("Expected failure to be cleared")
	}
	if db.latency != 0 {
		t.Error("Expected latency to be cleared")
	}
	if len(db.data) != 0 {
		t.Error("Expected data to be cleared")
	}
}

func TestMockTx(t *testing.T) {
	db := NewMockDB()
	tx := db.BeginTx()

	if tx.IsCommitted() {
		t.Error("Transaction should not be committed initially")
	}

	tx.Commit()
	if !tx.IsCommitted() {
		t.Error("Transaction should be committed")
	}

	tx2 := db.BeginTx()
	tx2.Rollback()
	if !tx2.IsRolledback() {
		t.Error("Transaction should be rolled back")
	}
}

func TestMockCache(t *testing.T) {
	cache := NewMockCache()
	ctx := context.Background()

	// Test Set and Get
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, found := cache.Get(ctx, "key1")
	if !found {
		t.Error("Expected to find cached value")
	}
	if value != "value1" {
		t.Error("Expected value1")
	}

	// Test TTL expiration
	err = cache.Set(ctx, "key2", "value2", 50*time.Millisecond)
	if err != nil {
		t.Errorf("Set with TTL failed: %v", err)
	}

	time.Sleep(60 * time.Millisecond)
	_, found = cache.Get(ctx, "key2")
	if found {
		t.Error("Should not find expired value")
	}

	// Test Delete
	cache.Set(ctx, "key3", "value3", 0)
	cache.Delete(ctx, "key3")
	_, found = cache.Get(ctx, "key3")
	if found {
		t.Error("Should not find deleted value")
	}
}

func TestMockCacheStats(t *testing.T) {
	cache := NewMockCache()
	ctx := context.Background()

	cache.Set(ctx, "key", "value", 0)
	cache.Get(ctx, "key")  // Hit
	cache.Get(ctx, "key")  // Hit
	cache.Get(ctx, "none") // Miss

	hits, misses := cache.Stats()
	if hits != 2 {
		t.Errorf("Expected 2 hits, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("Expected 1 miss, got %d", misses)
	}
}

func TestMockHTTPClient(t *testing.T) {
	client := NewMockHTTPClient()

	// Set response
	client.SetResponse("http://example.com/test", 200, `{"status":"ok"}`, map[string]string{
		"Content-Type": "application/json",
	})

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test requests recorded
	requests := client.GetRequests()
	if len(requests) != 1 {
		t.Errorf("Expected 1 request recorded, got %d", len(requests))
	}
}

func TestMockHTTPClientError(t *testing.T) {
	client := NewMockHTTPClient()
	expectedErr := errors.New("network error")
	client.SetError(expectedErr)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	_, err := client.Do(req)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockHTTPClientJSONResponse(t *testing.T) {
	client := NewMockHTTPClient()
	data := map[string]interface{}{"user": "test", "id": 123}
	client.SetJSONResponse("http://example.com/user", 200, data)

	req, _ := http.NewRequest("GET", "http://example.com/user", nil)
	resp, _ := client.Do(req)

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Error("Expected JSON content type")
	}
}

func TestMockMessageQueue(t *testing.T) {
	queue := NewMockMessageQueue()

	// Test Publish
	err := queue.Publish("topic1", []byte("message1"), nil)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	messages := queue.GetMessages("topic1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if string(messages[0].Body) != "message1" {
		t.Error("Message body mismatch")
	}
}

func TestMockMessageQueueSubscribe(t *testing.T) {
	queue := NewMockMessageQueue()
	ch := queue.Subscribe("topic1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		queue.Publish("topic1", []byte("async message"), nil)
	}()

	select {
	case msg := <-ch:
		if string(msg.Body) != "async message" {
			t.Error("Message body mismatch")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive message")
	}
}

func TestMockRows(t *testing.T) {
	columns := []string{"id", "name", "age"}
	data := [][]interface{}{
		{1, "John", 30},
		{2, "Jane", 25},
	}
	rows := NewMockRows(columns, data)

	cols, _ := rows.Columns()
	if len(cols) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(cols))
	}

	count := 0
	for rows.Next() {
		var id int
		var name string
		var age int
		rows.Scan(&id, &name, &age)
		count++
	}
	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}

	rows.Close()
}

func TestMockServer(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	server.Handle("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	resp, err := http.Get(server.URL() + "/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	requests := server.GetRequests()
	if len(requests) != 1 {
		t.Errorf("Expected 1 request, got %d", len(requests))
	}
}

func TestMockServerJSON(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	server.HandleJSON("/user", 200, map[string]interface{}{
		"id":   1,
		"name": "Test",
	})

	resp, err := http.Get(server.URL() + "/user")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["name"] != "Test" {
		t.Error("Expected name to be Test")
	}
}

// ==================== Fixture Tests ====================

func TestFixtureFactory(t *testing.T) {
	factory := NewFixtureFactory()

	// Test CreateUser
	user := factory.CreateUser()
	if user.ID == "" {
		t.Error("Expected user ID")
	}
	if user.Role != "user" {
		t.Error("Expected default role 'user'")
	}

	// Test with options
	adminUser := factory.CreateUser(WithUserRole("admin"), WithUserEmail("admin@test.com"))
	if adminUser.Role != "admin" {
		t.Error("Expected role 'admin'")
	}
	if adminUser.Email != "admin@test.com" {
		t.Error("Expected email 'admin@test.com'")
	}
}

func TestFixtureFactoryAccount(t *testing.T) {
	factory := NewFixtureFactory()
	user := factory.CreateUser()

	account := factory.CreateAccount(user.ID)
	if account.UserID != user.ID {
		t.Error("Expected account to belong to user")
	}
	if account.Balance != 1000.00 {
		t.Error("Expected default balance of 1000.00")
	}

	// With options
	account2 := factory.CreateAccount(user.ID, WithAccountBalance(5000), WithAccountCurrency("EUR"))
	if account2.Balance != 5000 {
		t.Error("Expected balance of 5000")
	}
	if account2.Currency != "EUR" {
		t.Error("Expected currency EUR")
	}
}

func TestFixtureFactoryTransaction(t *testing.T) {
	factory := NewFixtureFactory()
	user := factory.CreateUser()
	acc1 := factory.CreateAccount(user.ID)
	acc2 := factory.CreateAccount(user.ID)

	tx := factory.CreateTransaction(acc1.ID, acc2.ID)
	if tx.FromAccountID != acc1.ID {
		t.Error("Expected from account to match")
	}
	if tx.ToAccountID != acc2.ID {
		t.Error("Expected to account to match")
	}
	if tx.Amount != 100.00 {
		t.Error("Expected default amount of 100.00")
	}
}

func TestFixtureFactoryMerchant(t *testing.T) {
	factory := NewFixtureFactory()

	merchant := factory.CreateMerchant()
	if merchant.ID == "" {
		t.Error("Expected merchant ID")
	}
	if merchant.FeePercentage != 2.5 {
		t.Error("Expected default fee of 2.5%")
	}

	merchant2 := factory.CreateMerchant(WithMerchantFee(3.0), WithMerchantStatus("inactive"))
	if merchant2.FeePercentage != 3.0 {
		t.Error("Expected fee of 3.0%")
	}
	if merchant2.Status != "inactive" {
		t.Error("Expected status 'inactive'")
	}
}

func TestFixtureFactoryTestData(t *testing.T) {
	factory := NewFixtureFactory()
	data := factory.CreateTestData()

	if len(data.Users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(data.Users))
	}
	if len(data.Accounts) != 3 {
		t.Errorf("Expected 3 accounts, got %d", len(data.Accounts))
	}
	if len(data.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(data.Transactions))
	}
	if len(data.Merchants) != 1 {
		t.Errorf("Expected 1 merchant, got %d", len(data.Merchants))
	}
}

func TestFixtureManager(t *testing.T) {
	// Create temp fixtures directory
	tmpDir := t.TempDir()
	fixturesDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixturesDir, 0755)

	// Create a fixture file
	userFixture := map[string]interface{}{
		"id":    "usr_test",
		"email": "test@example.com",
	}
	data, _ := json.Marshal(userFixture)
	os.WriteFile(filepath.Join(fixturesDir, "user.json"), data, 0644)

	fm := NewFixtureManager(fixturesDir)
	err := fm.Load()
	if err != nil {
		t.Fatalf("Failed to load fixtures: %v", err)
	}

	user, found := fm.Get("user")
	if !found {
		t.Error("Expected to find user fixture")
	}

	userData := user.(map[string]interface{})
	if userData["id"] != "usr_test" {
		t.Error("Expected id 'usr_test'")
	}
}

func TestEnvironmentFixtures(t *testing.T) {
	env := NewEnvironmentFixtures()

	// Save original value
	original := os.Getenv("TEST_VAR")

	env.Set("TEST_VAR", "test_value")
	if os.Getenv("TEST_VAR") != "test_value" {
		t.Error("Expected TEST_VAR to be 'test_value'")
	}

	env.Restore()
	if os.Getenv("TEST_VAR") != original {
		t.Error("Expected TEST_VAR to be restored")
	}
}

func TestEnvironmentFixturesTestEnv(t *testing.T) {
	env := NewEnvironmentFixtures()
	env.SetTestEnvironment()
	defer env.Restore()

	if os.Getenv("ENV") != "test" {
		t.Error("Expected ENV to be 'test'")
	}
	if os.Getenv("LOG_LEVEL") != "debug" {
		t.Error("Expected LOG_LEVEL to be 'debug'")
	}
}

// ==================== Assertion Tests ====================

type mockT struct {
	errored    bool
	fataled    bool
	failedNow  bool
	lastMsg    string
}

func (m *mockT) Helper()                                  {}
func (m *mockT) Errorf(format string, args ...interface{}) { m.errored = true; m.lastMsg = format }
func (m *mockT) Fatalf(format string, args ...interface{}) { m.fataled = true }
func (m *mockT) FailNow()                                  { m.failedNow = true }

func TestAssertionsEqual(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.Equal(1, 1)
	if mt.errored {
		t.Error("Equal should not error for equal values")
	}

	a.Equal(1, 2)
	if !mt.errored {
		t.Error("Equal should error for unequal values")
	}
}

func TestAssertionsNil(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.Nil(nil)
	if mt.errored {
		t.Error("Nil should not error for nil")
	}

	mt.errored = false
	a.Nil("not nil")
	if !mt.errored {
		t.Error("Nil should error for non-nil")
	}
}

func TestAssertionsNoError(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.NoError(nil)
	if mt.errored {
		t.Error("NoError should not error for nil error")
	}

	mt.errored = false
	a.NoError(errors.New("error"))
	if !mt.errored {
		t.Error("NoError should error for non-nil error")
	}
}

func TestAssertionsContains(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.Contains("hello world", "world")
	if mt.errored {
		t.Error("Contains should not error when substring exists")
	}

	mt.errored = false
	a.Contains("hello world", "foo")
	if !mt.errored {
		t.Error("Contains should error when substring doesn't exist")
	}
}

func TestAssertionsLen(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.Len([]int{1, 2, 3}, 3)
	if mt.errored {
		t.Error("Len should not error for correct length")
	}

	mt.errored = false
	a.Len([]int{1, 2}, 3)
	if !mt.errored {
		t.Error("Len should error for incorrect length")
	}
}

func TestAssertionsComparison(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	a.Greater(5, 3)
	if mt.errored {
		t.Error("Greater should not error when first > second")
	}

	mt.errored = false
	a.Less(3, 5)
	if mt.errored {
		t.Error("Less should not error when first < second")
	}
}

func TestAssertionsEventually(t *testing.T) {
	mt := &mockT{}
	a := NewAssertions(mt)

	counter := 0
	a.Eventually(func() bool {
		counter++
		return counter >= 3
	}, 100*time.Millisecond, 10*time.Millisecond)

	if mt.errored {
		t.Error("Eventually should not error when condition is met")
	}
}

func TestHTTPAssertions(t *testing.T) {
	mt := &mockT{}
	h := NewHTTPAssertions(mt)

	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	h.StatusCodeRecorder(w, 200)
	if mt.errored {
		t.Error("StatusCodeRecorder should not error for correct status")
	}

	mt.errored = false
	h.StatusCodeRecorder(w, 404)
	if !mt.errored {
		t.Error("StatusCodeRecorder should error for incorrect status")
	}
}

func TestRequire(t *testing.T) {
	mt := &mockT{}
	r := NewRequire(mt)

	r.Equal(1, 1)
	if mt.failedNow {
		t.Error("Require should not fail for equal values")
	}

	r.Equal(1, 2)
	if !mt.failedNow {
		t.Error("Require should fail immediately for unequal values")
	}
}

// ==================== Integration Tests ====================

func TestIntegrationSuite(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Teardown()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	suite.SetupServer(handler)

	resp, err := suite.GET("/test", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	suite.AssertStatus(resp, http.StatusOK)
}

func TestIntegrationSuiteAPI(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Teardown()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	suite.SetupServer(handler)

	var result map[string]string
	suite.API().
		Get("/test").
		WithHeader("Accept", "application/json").
		Expect().
		Status(200).
		HeaderContains("Content-Type", "json").
		JSON(&result).
		End()

	if result["status"] != "ok" {
		t.Error("Expected status 'ok'")
	}
}

func TestIntegrationSuitePOST(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Teardown()

	var receivedBody map[string]interface{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
	})

	suite.SetupServer(handler)

	resp, _ := suite.POST("/create", map[string]interface{}{"name": "test"}, nil)
	defer resp.Body.Close()

	suite.AssertStatus(resp, http.StatusCreated)
	if receivedBody["name"] != "test" {
		t.Error("Expected body to contain name")
	}
}

func TestIntegrationSuiteWaitFor(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Teardown()

	counter := 0
	result := suite.WaitFor(func() bool {
		counter++
		return counter >= 3
	}, 100*time.Millisecond)

	if !result {
		t.Error("WaitFor should return true when condition is met")
	}
}

func TestRunSubTests(t *testing.T) {
	tests := []SubTest[int]{
		{Name: "test1", Data: 1},
		{Name: "test2", Data: 2},
	}

	ran := 0
	RunSubTests(t, tests, func(t *testing.T, data int) {
		ran++
	})

	if ran != 2 {
		t.Errorf("Expected 2 tests to run, got %d", ran)
	}
}

func TestRunTableTests(t *testing.T) {
	setupRan := 0
	teardownRan := 0

	tests := []TableTest{
		{
			Name:  "test1",
			Setup: func() { setupRan++ },
			Run: func(t *testing.T) {
				// Test logic
			},
			Teardown: func() { teardownRan++ },
		},
		{
			Name: "skipped",
			Skip: true,
			Run: func(t *testing.T) {
				t.Error("Should not run")
			},
		},
	}

	RunTableTests(t, tests)

	if setupRan != 1 {
		t.Errorf("Expected setup to run once, got %d", setupRan)
	}
	if teardownRan != 1 {
		t.Errorf("Expected teardown to run once, got %d", teardownRan)
	}
}
