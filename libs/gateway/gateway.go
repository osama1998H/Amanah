package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"amanah/libs/auth"
	"amanah/libs/middleware"
)

// ServiceConfig holds configuration for a backend service
type ServiceConfig struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	HealthEndpoint string            `json:"health_endpoint"`
	Timeout        time.Duration     `json:"timeout"`
	MaxRetries     int               `json:"max_retries"`
	CircuitBreaker *CircuitConfig    `json:"circuit_breaker,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
}

// RouteConfig holds configuration for a route
type RouteConfig struct {
	Path        string   `json:"path"`
	Methods     []string `json:"methods,omitempty"`
	Service     string   `json:"service"`
	StripPrefix string   `json:"strip_prefix,omitempty"`
	Rewrite     string   `json:"rewrite,omitempty"`
	RequireAuth bool     `json:"require_auth"`
	RateLimit   int      `json:"rate_limit,omitempty"` // Requests per minute
	Roles       []string `json:"roles,omitempty"`      // Required roles
	Version     string   `json:"version,omitempty"`    // API version
	Deprecated  bool     `json:"deprecated"`
	Sunset      string   `json:"sunset,omitempty"` // Deprecation date
}

// GatewayConfig holds the gateway configuration
type GatewayConfig struct {
	Services        map[string]*ServiceConfig
	Routes          []*RouteConfig
	DefaultTimeout  time.Duration
	RequestTimeout  time.Duration
	EnableMetrics   bool
	EnableLogging   bool
	CORSConfig      *middleware.SecurityConfig
	RateLimitConfig *middleware.RateLimitConfig
}

// DefaultGatewayConfig returns default gateway configuration
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		Services:       make(map[string]*ServiceConfig),
		Routes:         make([]*RouteConfig, 0),
		DefaultTimeout: 30 * time.Second,
		RequestTimeout: 60 * time.Second,
		EnableMetrics:  true,
		EnableLogging:  true,
	}
}

// Gateway is the API gateway
type Gateway struct {
	config          *GatewayConfig
	services        map[string]*serviceProxy
	routes          []*routeHandler
	healthChecker   *HealthChecker
	jwtManager      *auth.JWTManager
	apiKeyManager   *auth.APIKeyManager
	rbacManager     *auth.RBACManager
	rateLimiter     *middleware.RateLimiter
	mu              sync.RWMutex
	requestLogger   RequestLogger
}

// serviceProxy wraps a service with its proxy and circuit breaker
type serviceProxy struct {
	config    *ServiceConfig
	proxy     *httputil.ReverseProxy
	breaker   *CircuitBreaker
	targetURL *url.URL
}

// routeHandler handles a specific route
type routeHandler struct {
	config  *RouteConfig
	service *serviceProxy
	limiter *middleware.RateLimiter
}

// RequestLogger logs requests
type RequestLogger interface {
	LogRequest(ctx context.Context, req *RequestLog)
	LogResponse(ctx context.Context, resp *ResponseLog)
}

// RequestLog holds request logging data
type RequestLog struct {
	RequestID   string            `json:"request_id"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       string            `json:"query,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	ClientIP    string            `json:"client_ip"`
	Service     string            `json:"service"`
	Timestamp   time.Time         `json:"timestamp"`
}

// ResponseLog holds response logging data
type ResponseLog struct {
	RequestID    string        `json:"request_id"`
	StatusCode   int           `json:"status_code"`
	Duration     time.Duration `json:"duration"`
	BytesSent    int64         `json:"bytes_sent"`
	Error        string        `json:"error,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// NewGateway creates a new API gateway
func NewGateway(config *GatewayConfig) (*Gateway, error) {
	if config == nil {
		config = DefaultGatewayConfig()
	}

	g := &Gateway{
		config:        config,
		services:      make(map[string]*serviceProxy),
		routes:        make([]*routeHandler, 0),
		healthChecker: NewHealthChecker(10 * time.Second),
		jwtManager:    auth.NewJWTManager(nil),
		apiKeyManager: auth.NewAPIKeyManager(),
		rbacManager:   auth.NewRBACManager(),
	}

	if config.RateLimitConfig != nil {
		g.rateLimiter = middleware.NewRateLimiter(config.RateLimitConfig)
	}

	// Initialize services
	for name, svcConfig := range config.Services {
		if err := g.addService(name, svcConfig); err != nil {
			return nil, fmt.Errorf("failed to add service %s: %w", name, err)
		}
	}

	// Initialize routes
	for _, routeConfig := range config.Routes {
		if err := g.addRoute(routeConfig); err != nil {
			return nil, fmt.Errorf("failed to add route %s: %w", routeConfig.Path, err)
		}
	}

	return g, nil
}

// addService adds a backend service
func (g *Gateway) addService(name string, config *ServiceConfig) error {
	targetURL, err := url.Parse(config.URL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize director to add headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		for k, v := range config.Headers {
			req.Header.Set(k, v)
		}
	}

	// Set timeouts
	timeout := config.Timeout
	if timeout == 0 {
		timeout = g.config.DefaultTimeout
	}
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: timeout,
	}

	svc := &serviceProxy{
		config:    config,
		proxy:     proxy,
		targetURL: targetURL,
	}

	// Add circuit breaker if configured
	if config.CircuitBreaker != nil {
		svc.breaker = NewCircuitBreaker(config.CircuitBreaker)
	}

	g.services[name] = svc

	// Register with health checker
	healthURL := config.URL
	if config.HealthEndpoint != "" {
		healthURL = config.URL + config.HealthEndpoint
	}
	g.healthChecker.AddService(name, healthURL)

	return nil
}

// addRoute adds a route handler
func (g *Gateway) addRoute(config *RouteConfig) error {
	svc, exists := g.services[config.Service]
	if !exists {
		return fmt.Errorf("service %s not found", config.Service)
	}

	route := &routeHandler{
		config:  config,
		service: svc,
	}

	// Create per-route rate limiter if specified
	if config.RateLimit > 0 {
		route.limiter = middleware.NewRateLimiter(&middleware.RateLimitConfig{
			RequestsPerSecond: float64(config.RateLimit) / 60.0,
			BurstSize:         config.RateLimit / 6, // Allow burst of 10 seconds worth
			CleanupInterval:   time.Minute,
			KeyFunc:           middleware.DefaultKeyFunc,
		})
	}

	g.routes = append(g.routes, route)
	return nil
}

// ServeHTTP implements http.Handler
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}
	w.Header().Set("X-Request-ID", requestID)

	// Find matching route
	route := g.findRoute(r)
	if route == nil {
		g.writeError(w, http.StatusNotFound, "route_not_found", "No route matches the request")
		return
	}

	// Log request
	if g.config.EnableLogging && g.requestLogger != nil {
		g.requestLogger.LogRequest(r.Context(), &RequestLog{
			RequestID: requestID,
			Method:    r.Method,
			Path:      r.URL.Path,
			Query:     r.URL.RawQuery,
			ClientIP:  getClientIP(r),
			Service:   route.config.Service,
			Timestamp: start,
		})
	}

	// Check deprecation
	if route.config.Deprecated {
		w.Header().Set("Deprecation", "true")
		if route.config.Sunset != "" {
			w.Header().Set("Sunset", route.config.Sunset)
		}
		w.Header().Set("X-Deprecated-Message", "This API version is deprecated. Please upgrade to a newer version.")
	}

	// Set version header
	if route.config.Version != "" {
		w.Header().Set("X-API-Version", route.config.Version)
	}

	// Apply rate limiting
	if route.limiter != nil {
		clientKey := middleware.DefaultKeyFunc(r)
		if !route.limiter.Allow(clientKey) {
			g.writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
			return
		}
	} else if g.rateLimiter != nil {
		clientKey := middleware.DefaultKeyFunc(r)
		if !g.rateLimiter.Allow(clientKey) {
			g.writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
			return
		}
	}

	// Check authentication if required
	if route.config.RequireAuth {
		claims, err := g.authenticate(r)
		if err != nil {
			g.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}

		// Check roles if specified
		if len(route.config.Roles) > 0 && claims != nil {
			if !hasAnyRole(claims.Roles, route.config.Roles) {
				g.writeError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
				return
			}
		}
	}

	// Check circuit breaker
	if route.service.breaker != nil {
		if !route.service.breaker.Allow() {
			g.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Service temporarily unavailable")
			return
		}
	}

	// Modify request path if needed
	originalPath := r.URL.Path
	if route.config.StripPrefix != "" {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, route.config.StripPrefix)
	}
	if route.config.Rewrite != "" {
		r.URL.Path = route.config.Rewrite
	}

	// Proxy the request
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	route.service.proxy.ServeHTTP(rw, r)

	// Restore original path for logging
	r.URL.Path = originalPath

	// Record circuit breaker result
	if route.service.breaker != nil {
		if rw.statusCode >= 500 {
			route.service.breaker.RecordFailure()
		} else {
			route.service.breaker.RecordSuccess()
		}
	}

	// Log response
	if g.config.EnableLogging && g.requestLogger != nil {
		g.requestLogger.LogResponse(r.Context(), &ResponseLog{
			RequestID:  requestID,
			StatusCode: rw.statusCode,
			Duration:   time.Since(start),
			BytesSent:  rw.bytesWritten,
			Timestamp:  time.Now(),
		})
	}
}

// findRoute finds the matching route for a request
func (g *Gateway) findRoute(r *http.Request) *routeHandler {
	path := r.URL.Path
	method := r.Method

	// Check for versioned routes first
	version := r.Header.Get("X-API-Version")
	if version == "" {
		version = extractVersionFromPath(path)
	}

	for _, route := range g.routes {
		// Check version match
		if route.config.Version != "" && route.config.Version != version && version != "" {
			continue
		}

		// Check path match
		if !matchPath(route.config.Path, path) {
			continue
		}

		// Check method match
		if len(route.config.Methods) > 0 && !containsString(route.config.Methods, method) {
			continue
		}

		return route
	}

	// Fall back to routes without version
	for _, route := range g.routes {
		if route.config.Version != "" {
			continue
		}
		if !matchPath(route.config.Path, path) {
			continue
		}
		if len(route.config.Methods) > 0 && !containsString(route.config.Methods, method) {
			continue
		}
		return route
	}

	return nil
}

// authenticate authenticates the request
func (g *Gateway) authenticate(r *http.Request) (*auth.Claims, error) {
	// Try API key first
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		key, err := g.apiKeyManager.ValidateAPIKey(apiKey)
		if err != nil {
			return nil, err
		}
		// Create pseudo-claims from API key
		return &auth.Claims{
			Subject: key.ServiceID,
			Roles:   key.Roles,
		}, nil
	}

	// Try JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authentication provided")
	}

	token, err := auth.ExtractTokenFromHeader(authHeader)
	if err != nil {
		return nil, err
	}

	return g.jwtManager.ValidateToken(token)
}

// Handler returns the gateway as an http.Handler with all middleware applied
func (g *Gateway) Handler() http.Handler {
	var handler http.Handler = g

	// Apply security middleware
	if g.config.CORSConfig != nil {
		sm := middleware.NewSecurityMiddleware(g.config.CORSConfig)
		handler = sm.Handler()(handler)
	}

	// Apply request ID middleware
	handler = middleware.RequestIDMiddleware()(handler)

	return handler
}

// GetHealthChecker returns the health checker
func (g *Gateway) GetHealthChecker() *HealthChecker {
	return g.healthChecker
}

// SetRequestLogger sets the request logger
func (g *Gateway) SetRequestLogger(logger RequestLogger) {
	g.requestLogger = logger
}

// SetJWTManager sets the JWT manager
func (g *Gateway) SetJWTManager(manager *auth.JWTManager) {
	g.jwtManager = manager
}

// SetAPIKeyManager sets the API key manager
func (g *Gateway) SetAPIKeyManager(manager *auth.APIKeyManager) {
	g.apiKeyManager = manager
}

// SetRBACManager sets the RBAC manager
func (g *Gateway) SetRBACManager(manager *auth.RBACManager) {
	g.rbacManager = manager
}

// responseWriter wraps http.ResponseWriter to capture status and bytes
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// writeError writes a JSON error response
func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

// Helper functions

func matchPath(pattern, path string) bool {
	// Simple prefix matching with wildcard support
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	return path == pattern || strings.HasPrefix(path, pattern+"/")
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func hasAnyRole(userRoles, requiredRoles []string) bool {
	for _, userRole := range userRoles {
		for _, requiredRole := range requiredRoles {
			if userRole == requiredRole {
				return true
			}
		}
	}
	return false
}

func extractVersionFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && strings.HasPrefix(parts[0], "v") {
		return parts[0]
	}
	return ""
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
