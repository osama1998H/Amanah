package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===== RegisterRoutes Tests =====

func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux)

	// Verify /transactions/ route is registered
	req := httptest.NewRequest(http.MethodGet, "/transactions/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Route should be registered (may return bad gateway since no backend)
	// but not 404
	if rr.Code == http.StatusNotFound {
		t.Error("expected /transactions/ route to be registered")
	}
}

func TestRegisterRoutes_UnknownPath(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/unknown/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Unknown paths should return 404
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for unknown path, got %d", rr.Code)
	}
}

// ===== Proxy Function Tests =====

func TestProxy_Success(t *testing.T) {
	// Create a mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "mock")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body, _ := io.ReadAll(rr.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", string(body))
	}

	if rr.Header().Get("X-Backend") != "mock" {
		t.Error("expected X-Backend header from backend")
	}
}

func TestProxy_ForwardsMethod(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var receivedMethod string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			handler := proxy(backend.URL)

			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if receivedMethod != method {
				t.Errorf("expected method %s, got %s", method, receivedMethod)
			}
		})
	}
}

func TestProxy_ForwardsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Error("expected X-Custom-Header to be forwarded")
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Error("expected Authorization header to be forwarded")
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type header to be forwarded")
	}
}

func TestProxy_ForwardsBody(t *testing.T) {
	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	requestBody := `{"key":"value","nested":{"foo":"bar"}}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedBody != requestBody {
		t.Errorf("expected body %s, got %s", requestBody, receivedBody)
	}
}

func TestProxy_ForwardsQueryParams(t *testing.T) {
	var receivedQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=qux", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedQuery != "foo=bar&baz=qux" {
		t.Errorf("expected query foo=bar&baz=qux, got %s", receivedQuery)
	}
}

func TestProxy_ForwardsPath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	tests := []string{
		"/",
		"/test",
		"/api/v1/users",
		"/deeply/nested/path/to/resource",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if receivedPath != path {
				t.Errorf("expected path %s, got %s", path, receivedPath)
			}
		})
	}
}

func TestProxy_BackendStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer backend.Close()

			handler := proxy(backend.URL)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != code {
				t.Errorf("expected status %d, got %d", code, rr.Code)
			}
		})
	}
}

func TestProxy_InvalidTarget(t *testing.T) {
	// Test with invalid URL scheme
	handler := proxy("://invalid-url")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status 502 for invalid target, got %d", rr.Code)
	}
}

func TestProxy_BackendUnavailable(t *testing.T) {
	// Use a URL that won't connect
	handler := proxy("http://127.0.0.1:59999")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// When backend is unavailable, reverse proxy returns 502
	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status 502 for unavailable backend, got %d", rr.Code)
	}
}

func TestProxy_LargeBody(t *testing.T) {
	var receivedLen int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedLen = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	// Create a 1MB body
	largeBody := make([]byte, 1024*1024)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(string(largeBody)))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if receivedLen != len(largeBody) {
		t.Errorf("expected body length %d, got %d", len(largeBody), receivedLen)
	}
}

func TestProxy_ResponseBody(t *testing.T) {
	responseBody := `{"message":"Hello from backend","status":"success"}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body, _ := io.ReadAll(rr.Body)
	if string(body) != responseBody {
		t.Errorf("expected response body %s, got %s", responseBody, string(body))
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type header from backend response")
	}
}

// ===== Integration-style Tests =====

func TestTransactionsRoute_Proxies(t *testing.T) {
	// Create a mock transaction service
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Service", "transaction")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"transaction"}`))
	}))
	defer backend.Close()

	// Create gateway mux and register with mock backend
	mux := http.NewServeMux()
	mux.HandleFunc("/transactions/", proxy(backend.URL))

	req := httptest.NewRequest(http.MethodGet, "/transactions/list", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("X-Service") != "transaction" {
		t.Error("expected request to be proxied to transaction service")
	}
}

func TestGateway_HealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body, _ := io.ReadAll(rr.Body)
	if string(body) != "OK" {
		t.Errorf("expected body OK, got %s", string(body))
	}
}

func TestGateway_MultipleRoutes(t *testing.T) {
	// Create mock services
	transactionBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("transaction"))
	}))
	defer transactionBackend.Close()

	accountBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("account"))
	}))
	defer accountBackend.Close()

	// Create gateway with multiple routes
	mux := http.NewServeMux()
	mux.HandleFunc("/transactions/", proxy(transactionBackend.URL))
	mux.HandleFunc("/accounts/", proxy(accountBackend.URL))

	tests := []struct {
		path     string
		expected string
	}{
		{"/transactions/list", "transaction"},
		{"/accounts/me", "account"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			body, _ := io.ReadAll(rr.Body)
			if string(body) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(body))
			}
		})
	}
}

// ===== Edge Cases =====

func TestProxy_EmptyResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodDelete, "/resource", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rr.Code)
	}

	body, _ := io.ReadAll(rr.Body)
	if len(body) != 0 {
		t.Errorf("expected empty body, got %s", string(body))
	}
}

func TestProxy_MultipleHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "cookie1=value1")
		w.Header().Add("Set-Cookie", "cookie2=value2")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	cookies := rr.Header()["Set-Cookie"]
	if len(cookies) < 2 {
		t.Errorf("expected multiple Set-Cookie headers, got %d", len(cookies))
	}
}

func TestProxy_ContentEncoding(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts encoding
		acceptEncoding := r.Header.Get("Accept-Encoding")
		if acceptEncoding != "" {
			w.Header().Set("X-Accept-Encoding", acceptEncoding)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify Accept-Encoding was forwarded
	if rr.Header().Get("X-Accept-Encoding") == "" {
		t.Error("expected Accept-Encoding to be forwarded")
	}
}

func TestProxy_SpecialCharactersInPath(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just acknowledge the request was received
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy(backend.URL)

	tests := []struct {
		name string
		path string
	}{
		{"spaces encoded", "/path%20with%20spaces"},
		{"special chars", "/path/with/special%2Fchar"},
		{"unicode", "/path/用户"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rr.Code)
			}
		})
	}
}
