package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"amanah/libs/auth"
)

func TestRateLimiterAllow(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
		KeyFunc:           DefaultKeyFunc,
	}

	limiter := NewRateLimiter(config)
	defer limiter.Close()

	// Should allow burst requests
	for i := 0; i < 5; i++ {
		if !limiter.Allow("client1") {
			t.Errorf("Request %d should be allowed within burst", i)
		}
	}

	// Next request should be denied (burst exhausted)
	if limiter.Allow("client1") {
		t.Error("Request after burst should be denied")
	}

	// Different client should be allowed
	if !limiter.Allow("client2") {
		t.Error("Different client should be allowed")
	}
}

func TestRateLimiterMiddleware(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
		KeyFunc:           func(r *http.Request) string { return "test" },
	}

	limiter := NewRateLimiter(config)
	defer limiter.Close()

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i, rr.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", rr.Code)
	}

	// Check rate limit headers
	if rr.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	config := &SlidingWindowConfig{
		WindowSize:      100 * time.Millisecond,
		MaxRequests:     3,
		CleanupInterval: time.Second,
		KeyFunc:         DefaultKeyFunc,
	}

	limiter := NewSlidingWindowLimiter(config)
	defer limiter.Close()

	// Should allow max requests
	for i := 0; i < 3; i++ {
		if !limiter.Allow("client") {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// Fourth request should be denied
	if limiter.Allow("client") {
		t.Error("Fourth request should be denied")
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Should allow again
	if !limiter.Allow("client") {
		t.Error("Request after window reset should be allowed")
	}
}

func TestSecurityMiddlewareHeaders(t *testing.T) {
	config := DefaultSecurityConfig()
	sm := NewSecurityMiddleware(config)

	handler := sm.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check security headers
	headers := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}

	for header, expected := range headers {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("Header %s: expected '%s', got '%s'", header, expected, got)
		}
	}
}

func TestSecurityMiddlewareCORS(t *testing.T) {
	config := DefaultSecurityConfig()
	config.AllowedOrigins = []string{"https://example.com", "*.example.org"}
	sm := NewSecurityMiddleware(config)

	handler := sm.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		origin         string
		expectCORS     bool
		expectPreflight bool
	}{
		{"allowed origin", "https://example.com", true, false},
		{"wildcard subdomain", "https://sub.example.org", true, false},
		{"disallowed origin", "https://evil.com", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Origin", tt.origin)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			hasOrigin := rr.Header().Get("Access-Control-Allow-Origin") != ""
			if hasOrigin != tt.expectCORS {
				t.Errorf("Expected CORS=%v, got %v", tt.expectCORS, hasOrigin)
			}
		})
	}
}

func TestSecurityMiddlewareCORSPreflight(t *testing.T) {
	config := DefaultSecurityConfig()
	config.AllowedOrigins = []string{"https://example.com"}
	sm := NewSecurityMiddleware(config)

	handler := sm.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204 for preflight, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Expected Access-Control-Allow-Methods header")
	}
}

func TestInputValidationMiddleware(t *testing.T) {
	iv := NewInputValidationMiddleware(1024) // 1KB limit

	handler := iv.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Valid content type
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid content type, got %d", rr.Code)
	}

	// Invalid content type
	req = httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "text/html")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected 415 for invalid content type, got %d", rr.Code)
	}
}

func TestSQLInjectionMiddleware(t *testing.T) {
	sqli := NewSQLInjectionMiddleware()

	handler := sqli.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		key    string
		value  string
		expect int
	}{
		{"normal query", "name", "john", http.StatusOK},
		{"sql injection attempt", "id", "1' OR '1'='1", http.StatusBadRequest},
		{"union injection with quote", "id", "'union select", http.StatusBadRequest},
		{"comment injection", "id", "1--", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			q := req.URL.Query()
			q.Set(tt.key, tt.value)
			req.URL.RawQuery = q.Encode()
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expect {
				t.Errorf("Expected %d, got %d", tt.expect, rr.Code)
			}
		})
	}
}

func TestXSSMiddleware(t *testing.T) {
	xss := NewXSSMiddleware()

	handler := xss.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		key    string
		value  string
		expect int
	}{
		{"normal query", "name", "john", http.StatusOK},
		{"script tag", "input", "<script>alert('xss')</script>", http.StatusBadRequest},
		{"javascript protocol", "url", "javascript:alert(1)", http.StatusBadRequest},
		{"event handler", "attr", "onclick=alert(1)", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			q := req.URL.Query()
			q.Set(tt.key, tt.value)
			req.URL.RawQuery = q.Encode()
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expect {
				t.Errorf("Expected %d, got %d", tt.expect, rr.Code)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	jwtManager := auth.NewJWTManager(&auth.JWTConfig{
		Secret:           "test-secret",
		AccessExpiration: time.Hour,
	})

	config := &AuthConfig{
		JWTManager: jwtManager,
		AllowJWT:   true,
		SkipPaths:  []string{"/health", "/public"},
	}

	am := NewAuthMiddleware(config)

	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		w.Write([]byte(userID))
	}))

	// Test skip path
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Health check should pass without auth, got %d", rr.Code)
	}

	// Test missing auth
	req = httptest.NewRequest("GET", "/protected", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for missing auth, got %d", rr.Code)
	}

	// Test valid JWT
	token, _ := jwtManager.GenerateAccessToken("user123", "test@example.com", "Test", []string{"user"})
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid JWT, got %d", rr.Code)
	}

	if rr.Body.String() != "user123" {
		t.Errorf("Expected user ID 'user123', got '%s'", rr.Body.String())
	}
}

func TestAuthMiddlewareAPIKey(t *testing.T) {
	apiKeyManager := auth.NewAPIKeyManager()
	rawKey, _, _ := apiKeyManager.GenerateAPIKey("Test", "", "", []string{"service"}, 0, nil)

	config := &AuthConfig{
		APIKeyManager: apiKeyManager,
		AllowAPIKey:   true,
		SkipPaths:     []string{"/health"},
	}

	am := NewAuthMiddleware(config)

	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := GetAPIKey(r.Context())
		if apiKey != nil {
			w.Write([]byte(apiKey.Name))
		}
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-API-Key", rawKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid API key, got %d", rr.Code)
	}

	if rr.Body.String() != "Test" {
		t.Errorf("Expected API key name 'Test', got '%s'", rr.Body.String())
	}
}

func TestRequireRoles(t *testing.T) {
	rbac := auth.NewRBACManager()

	handler := RequireRoles(rbac, "admin", "superadmin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test without roles
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 without roles, got %d", rr.Code)
	}

	// Test with wrong role
	ctx := context.WithValue(context.Background(), UserRolesKey, []string{"user"})
	req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 with wrong role, got %d", rr.Code)
	}

	// Test with correct role
	ctx = context.WithValue(context.Background(), UserRolesKey, []string{"admin"})
	req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with correct role, got %d", rr.Code)
	}
}

func TestRequirePermission(t *testing.T) {
	rbac := auth.NewRBACManager()

	handler := RequirePermission(rbac, auth.PermAdminAccess)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test with user role (no admin permission)
	ctx := context.WithValue(context.Background(), UserRolesKey, []string{"user"})
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 without permission, got %d", rr.Code)
	}

	// Test with admin role
	ctx = context.WithValue(context.Background(), UserRolesKey, []string{"admin"})
	req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with permission, got %d", rr.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		w.Write([]byte(requestID))
	}))

	// Test auto-generated ID
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header")
	}

	// Test existing ID is preserved
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "custom-id")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") != "custom-id" {
		t.Error("Expected existing request ID to be preserved")
	}
}

func TestChainMiddleware(t *testing.T) {
	order := []string{}

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	handler := ChainMiddleware(m1, m2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("At position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, UserIDKey, "user123")
	ctx = context.WithValue(ctx, UserEmailKey, "test@example.com")
	ctx = context.WithValue(ctx, UserNameKey, "Test User")
	ctx = context.WithValue(ctx, UserRolesKey, []string{"user", "admin"})
	ctx = context.WithValue(ctx, TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, RequestIDKey, "req-123")

	if GetUserID(ctx) != "user123" {
		t.Error("GetUserID failed")
	}
	if GetUserEmail(ctx) != "test@example.com" {
		t.Error("GetUserEmail failed")
	}
	if GetUserName(ctx) != "Test User" {
		t.Error("GetUserName failed")
	}
	roles := GetUserRoles(ctx)
	if len(roles) != 2 {
		t.Error("GetUserRoles failed")
	}
	if GetTenantID(ctx) != "tenant-1" {
		t.Error("GetTenantID failed")
	}
	if GetRequestID(ctx) != "req-123" {
		t.Error("GetRequestID failed")
	}

	// Test nil/empty context
	emptyCtx := context.Background()
	if GetUserID(emptyCtx) != "" {
		t.Error("GetUserID should return empty for missing value")
	}
	if GetUserRoles(emptyCtx) != nil {
		t.Error("GetUserRoles should return nil for missing value")
	}
}

func TestDefaultKeyFunc(t *testing.T) {
	tests := []struct {
		name            string
		xForwardedFor   string
		xRealIP         string
		remoteAddr      string
		expectedContains string
	}{
		{"X-Forwarded-For takes precedence", "10.0.0.1", "10.0.0.2", "10.0.0.3:1234", "10.0.0.1"},
		{"X-Real-IP second priority", "", "10.0.0.2", "10.0.0.3:1234", "10.0.0.2"},
		{"RemoteAddr fallback", "", "", "10.0.0.3:1234", "10.0.0.3:1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			key := DefaultKeyFunc(req)
			if key != tt.expectedContains {
				t.Errorf("Expected key to be %s, got %s", tt.expectedContains, key)
			}
		})
	}
}

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(10, 2) // 10 max tokens, 2 per second

	// Should allow up to 10 requests immediately
	for i := 0; i < 10; i++ {
		if !tb.Allow() {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// 11th should be denied
	if tb.Allow() {
		t.Error("11th request should be denied")
	}

	// Wait for refill
	time.Sleep(600 * time.Millisecond) // Should refill ~1.2 tokens

	// Should allow one more
	if !tb.Allow() {
		t.Error("Request after refill should be allowed")
	}

	// Check tokens count
	tokens := tb.Tokens()
	if tokens < 0 || tokens > 10 {
		t.Errorf("Tokens should be between 0 and 10, got %f", tokens)
	}
}
