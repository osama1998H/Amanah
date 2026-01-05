package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{
		MaxFailures:      3,
		Timeout:          time.Second,
		HalfOpenRequests: 2,
		SuccessThreshold: 2,
	})

	// Initially closed
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed, got %v", cb.State())
	}

	// Should allow requests
	for i := 0; i < 10; i++ {
		if !cb.Allow() {
			t.Error("Should allow requests when closed")
		}
		cb.RecordSuccess()
	}

	// Still closed
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after successes, got %v", cb.State())
	}
}

func TestCircuitBreakerOpenOnFailures(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{
		MaxFailures:      3,
		Timeout:          time.Second,
		HalfOpenRequests: 2,
		SuccessThreshold: 2,
	})

	// Record failures until circuit opens
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Error("Should allow requests when closed")
		}
		cb.RecordFailure()
	}

	// Should be open now
	if cb.State() != StateOpen {
		t.Errorf("Expected StateOpen after %d failures, got %v", cb.Failures(), cb.State())
	}

	// Should not allow requests
	if cb.Allow() {
		t.Error("Should not allow requests when open")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{
		MaxFailures:      2,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 2,
		SuccessThreshold: 2,
	})

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should transition to half-open
	if !cb.Allow() {
		t.Error("Should allow first request after timeout")
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("Expected StateHalfOpen, got %v", cb.State())
	}

	// Allow limited requests in half-open
	if !cb.Allow() {
		t.Error("Should allow second request in half-open")
	}

	// Third should be blocked
	if cb.Allow() {
		t.Error("Should block third request in half-open")
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{
		MaxFailures:      2,
		Timeout:          50 * time.Millisecond,
		HalfOpenRequests: 3,
		SuccessThreshold: 2,
	})

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// First request in half-open
	cb.Allow()
	cb.RecordSuccess()
	cb.Allow()
	cb.RecordSuccess()

	// Should be closed now
	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{MaxFailures: 2})

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected StateClosed after reset, got %v", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestCircuitBreakerStateChangeCallback(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{MaxFailures: 2})

	transitions := make([]string, 0)
	cb.OnStateChange(func(from, to CircuitState) {
		transitions = append(transitions, from.String()+"->"+to.String())
	})

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for callback
	time.Sleep(10 * time.Millisecond)

	if len(transitions) != 1 || transitions[0] != "closed->open" {
		t.Errorf("Expected transition closed->open, got %v", transitions)
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitConfig{MaxFailures: 5})

	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordSuccess()

	stats := cb.Stats()
	if stats.State != "closed" {
		t.Errorf("Expected state 'closed', got '%s'", stats.State)
	}
	if stats.Failures != 0 { // Reset on success
		t.Errorf("Expected 0 failures (reset on success), got %d", stats.Failures)
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	cb1 := registry.Get("service1", nil)
	cb2 := registry.Get("service2", nil)
	cb1Again := registry.Get("service1", nil)

	if cb1 != cb1Again {
		t.Error("Should return same circuit breaker for same service")
	}

	if cb1 == cb2 {
		t.Error("Different services should have different circuit breakers")
	}

	// Test stats
	cb1.Allow()
	cb1.RecordFailure()

	stats := registry.Stats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 services in stats, got %d", len(stats))
	}
}

func TestHealthChecker(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	hc := NewHealthChecker(100 * time.Millisecond)
	hc.AddService("test-service", server.URL+"/health")

	// Check all
	hc.CheckAll()

	// Get health
	health, exists := hc.GetServiceHealth("test-service")
	if !exists {
		t.Fatal("Service should exist")
	}

	if health.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", health.Status)
	}
}

func TestHealthCheckerUnhealthy(t *testing.T) {
	// Create a failing test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hc := NewHealthChecker(100 * time.Millisecond)
	hc.AddService("failing-service", server.URL+"/health")

	hc.CheckAll()

	health, _ := hc.GetServiceHealth("failing-service")
	if health.Status != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", health.Status)
	}
}

func TestHealthCheckerAggregated(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	hc := NewHealthChecker(100 * time.Millisecond)
	hc.AddService("healthy", healthy.URL)
	hc.AddService("unhealthy", unhealthy.URL)

	hc.CheckAll()

	agg := hc.GetAggregatedHealth()
	if agg.Healthy != 1 {
		t.Errorf("Expected 1 healthy, got %d", agg.Healthy)
	}
	if agg.Unhealthy != 1 {
		t.Errorf("Expected 1 unhealthy, got %d", agg.Unhealthy)
	}
	if agg.Status != HealthStatusUnhealthy {
		t.Errorf("Expected overall unhealthy status, got %s", agg.Status)
	}
}

func TestHealthCheckerStartStop(t *testing.T) {
	hc := NewHealthChecker(50 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hc.AddService("test", server.URL)
	hc.Start()

	// Wait for a check
	time.Sleep(100 * time.Millisecond)

	health, _ := hc.GetServiceHealth("test")
	if health.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy after start, got %s", health.Status)
	}

	hc.Stop()
}

func TestHealthHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hc := NewHealthChecker(time.Second)
	hc.AddService("test", server.URL)
	hc.CheckAll()

	handler := hc.HealthHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()

	req := httptest.NewRequest("GET", "/live", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"/api/v1/users", "/api/v1/users", true},
		{"/api/v1/users", "/api/v1/users/123", true},
		{"/api/v1/users/", "/api/v1/users/123", true},
		{"/api/*", "/api/v1/users", true},
		{"/api/*", "/api/", true},
		{"/api/v1/users", "/api/v2/users", false},
		{"/admin", "/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.path, func(t *testing.T) {
			result := matchPath(tt.pattern, tt.path)
			if result != tt.match {
				t.Errorf("matchPath(%s, %s) = %v, want %v", tt.pattern, tt.path, result, tt.match)
			}
		})
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	tests := []struct {
		path    string
		version string
	}{
		{"/v1/users", "v1"},
		{"/v2/api/users", "v2"},
		{"/api/users", ""},
		{"/users", ""},
		{"v1/users", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractVersionFromPath(tt.path)
			if result != tt.version {
				t.Errorf("extractVersionFromPath(%s) = %s, want %s", tt.path, result, tt.version)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"GET", "POST", "PUT"}

	if !containsString(slice, "GET") {
		t.Error("Should contain GET")
	}

	if containsString(slice, "DELETE") {
		t.Error("Should not contain DELETE")
	}
}

func TestHasAnyRole(t *testing.T) {
	userRoles := []string{"user", "merchant"}

	if !hasAnyRole(userRoles, []string{"admin", "merchant"}) {
		t.Error("Should match merchant role")
	}

	if hasAnyRole(userRoles, []string{"admin", "superadmin"}) {
		t.Error("Should not match any role")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		xri      string
		remote   string
		expected string
	}{
		{"X-Forwarded-For", "1.2.3.4, 5.6.7.8", "", "9.10.11.12:1234", "1.2.3.4"},
		{"X-Real-IP", "", "1.2.3.4", "9.10.11.12:1234", "1.2.3.4"},
		{"RemoteAddr", "", "", "9.10.11.12:1234", "9.10.11.12:1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			req.RemoteAddr = tt.remote

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("getClientIP() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestGatewayBasic(t *testing.T) {
	// Create a backend service
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "true")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	}))
	defer backend.Close()

	config := &GatewayConfig{
		Services: map[string]*ServiceConfig{
			"backend": {
				Name: "backend",
				URL:  backend.URL,
			},
		},
		Routes: []*RouteConfig{
			{
				Path:    "/api/",
				Service: "backend",
			},
		},
	}

	gw, err := NewGateway(config)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestGatewayRouteNotFound(t *testing.T) {
	config := &GatewayConfig{
		Services: map[string]*ServiceConfig{},
		Routes:   []*RouteConfig{},
	}

	gw, _ := NewGateway(config)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestGatewayDeprecationHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	config := &GatewayConfig{
		Services: map[string]*ServiceConfig{
			"backend": {Name: "backend", URL: backend.URL},
		},
		Routes: []*RouteConfig{
			{
				Path:       "/api/v1/",
				Service:    "backend",
				Version:    "v1",
				Deprecated: true,
				Sunset:     "2024-12-31",
			},
		},
	}

	gw, _ := NewGateway(config)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	gw.ServeHTTP(rr, req)

	if rr.Header().Get("Deprecation") != "true" {
		t.Error("Expected Deprecation header")
	}
	if rr.Header().Get("Sunset") != "2024-12-31" {
		t.Error("Expected Sunset header")
	}
	if rr.Header().Get("X-API-Version") != "v1" {
		t.Error("Expected X-API-Version header")
	}
}

func TestCircuitStateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}
