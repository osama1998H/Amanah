package middleware

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// SecurityConfig holds security middleware configuration
type SecurityConfig struct {
	// CORS settings
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds

	// Security headers
	HSTSMaxAge            int  // HSTS max-age in seconds (0 to disable)
	HSTSIncludeSubdomains bool // Include subdomains in HSTS
	HSTSPreload           bool // Enable HSTS preload
	FrameOptions          string // DENY, SAMEORIGIN, or ALLOW-FROM uri
	ContentTypeNosniff    bool   // Enable X-Content-Type-Options: nosniff
	XSSProtection         string // X-XSS-Protection header value
	ContentSecurityPolicy string // Content-Security-Policy header
	ReferrerPolicy        string // Referrer-Policy header
	PermissionsPolicy     string // Permissions-Policy header

	// Request validation
	MaxBodySize     int64    // Maximum request body size
	AllowedHosts    []string // Allowed Host header values
	TrustedProxies  []string // Trusted proxy IPs for X-Forwarded-* headers
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		AllowedOrigins:        []string{},
		AllowedMethods:        []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:        []string{"Accept", "Authorization", "Content-Type", "X-Requested-With", "X-API-Key"},
		ExposedHeaders:        []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials:      true,
		MaxAge:                86400, // 24 hours
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		FrameOptions:          "DENY",
		ContentTypeNosniff:    true,
		XSSProtection:         "1; mode=block",
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
		MaxBodySize:           10 * 1024 * 1024, // 10MB
		AllowedHosts:          []string{},
		TrustedProxies:        []string{"127.0.0.1", "::1"},
	}
}

// ProductionSecurityConfig returns security configuration suitable for production
func ProductionSecurityConfig() *SecurityConfig {
	config := DefaultSecurityConfig()
	config.HSTSPreload = true
	config.ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'"
	return config
}

// SecurityMiddleware provides security headers and protections
type SecurityMiddleware struct {
	config *SecurityConfig
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config *SecurityConfig) *SecurityMiddleware {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	return &SecurityMiddleware{config: config}
}

// Handler returns the HTTP middleware handler
func (sm *SecurityMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			sm.setSecurityHeaders(w, r)

			// Handle CORS
			if sm.handleCORS(w, r) {
				return // OPTIONS request handled
			}

			// Validate host
			if !sm.validateHost(r) {
				http.Error(w, "Invalid host", http.StatusBadRequest)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setSecurityHeaders sets all security-related headers
func (sm *SecurityMiddleware) setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// HSTS (only on HTTPS)
	if sm.config.HSTSMaxAge > 0 {
		hstsValue := "max-age=" + itoa(sm.config.HSTSMaxAge)
		if sm.config.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if sm.config.HSTSPreload {
			hstsValue += "; preload"
		}
		w.Header().Set("Strict-Transport-Security", hstsValue)
	}

	// Frame options
	if sm.config.FrameOptions != "" {
		w.Header().Set("X-Frame-Options", sm.config.FrameOptions)
	}

	// Content type sniffing
	if sm.config.ContentTypeNosniff {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}

	// XSS protection
	if sm.config.XSSProtection != "" {
		w.Header().Set("X-XSS-Protection", sm.config.XSSProtection)
	}

	// Content Security Policy
	if sm.config.ContentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", sm.config.ContentSecurityPolicy)
	}

	// Referrer Policy
	if sm.config.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", sm.config.ReferrerPolicy)
	}

	// Permissions Policy
	if sm.config.PermissionsPolicy != "" {
		w.Header().Set("Permissions-Policy", sm.config.PermissionsPolicy)
	}

	// Cache control for security-sensitive responses
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// handleCORS handles CORS preflight and regular requests
func (sm *SecurityMiddleware) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")

	if origin == "" {
		return false // Not a CORS request
	}

	// Check if origin is allowed
	allowed := sm.isOriginAllowed(origin)
	if !allowed {
		return false
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", origin)

	if sm.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	w.Header().Set("Access-Control-Expose-Headers", strings.Join(sm.config.ExposedHeaders, ", "))

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(sm.config.AllowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(sm.config.AllowedHeaders, ", "))
		w.Header().Set("Access-Control-Max-Age", itoa(sm.config.MaxAge))
		w.WriteHeader(http.StatusNoContent)
		return true
	}

	return false
}

// isOriginAllowed checks if the origin is in the allowed list
func (sm *SecurityMiddleware) isOriginAllowed(origin string) bool {
	if len(sm.config.AllowedOrigins) == 0 {
		return false
	}

	for _, allowed := range sm.config.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}

	return false
}

// validateHost checks if the Host header is allowed
func (sm *SecurityMiddleware) validateHost(r *http.Request) bool {
	if len(sm.config.AllowedHosts) == 0 {
		return true // No restriction
	}

	host := r.Host
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	for _, allowed := range sm.config.AllowedHosts {
		if allowed == host {
			return true
		}
		if allowed == "*" {
			return true
		}
	}

	return false
}

// InputValidationMiddleware provides input validation
type InputValidationMiddleware struct {
	maxBodySize int64
}

// NewInputValidationMiddleware creates a new input validation middleware
func NewInputValidationMiddleware(maxBodySize int64) *InputValidationMiddleware {
	if maxBodySize <= 0 {
		maxBodySize = 10 * 1024 * 1024 // 10MB default
	}
	return &InputValidationMiddleware{maxBodySize: maxBodySize}
}

// Handler returns the middleware handler
func (iv *InputValidationMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit body size
			r.Body = http.MaxBytesReader(w, r.Body, iv.maxBodySize)

			// Validate Content-Type for POST/PUT/PATCH
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				contentType := r.Header.Get("Content-Type")
				if contentType != "" && !iv.isValidContentType(contentType) {
					http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isValidContentType checks if content type is allowed
func (iv *InputValidationMiddleware) isValidContentType(contentType string) bool {
	// Extract mime type (ignore charset and other parameters)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	allowedTypes := []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
		"text/plain",
	}

	for _, allowed := range allowedTypes {
		if strings.EqualFold(contentType, allowed) {
			return true
		}
	}

	return false
}

// SQLInjectionMiddleware provides basic SQL injection protection
type SQLInjectionMiddleware struct {
	patterns []*regexp.Regexp
}

// NewSQLInjectionMiddleware creates a new SQL injection middleware
func NewSQLInjectionMiddleware() *SQLInjectionMiddleware {
	patterns := []string{
		`(?i)(\%27)|(\')|(\-\-)|(\%23)|(#)`,
		`(?i)((\%3D)|(=))[^\n]*((\%27)|(\')|(\-\-)|(\%3B)|(;))`,
		`(?i)\w*((\%27)|(\'))((\%6F)|o|(\%4F))((\%72)|r|(\%52))`,
		`(?i)((\%27)|(\'))union`,
		`(?i)exec(\s|\+)+(s|x)p\w+`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SQLInjectionMiddleware{patterns: compiled}
}

// Handler returns the middleware handler
func (sqli *SQLInjectionMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check query parameters
			for _, values := range r.URL.Query() {
				for _, value := range values {
					if sqli.isSuspicious(value) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{
							"error":   "invalid_request",
							"message": "Request contains potentially malicious content",
						})
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSuspicious checks if input matches SQL injection patterns
func (sqli *SQLInjectionMiddleware) isSuspicious(input string) bool {
	for _, pattern := range sqli.patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// XSSMiddleware provides basic XSS protection
type XSSMiddleware struct {
	patterns []*regexp.Regexp
}

// NewXSSMiddleware creates a new XSS middleware
func NewXSSMiddleware() *XSSMiddleware {
	patterns := []string{
		`(?i)<script[^>]*>.*?</script>`,
		`(?i)<script[^>]*>`,
		`(?i)javascript:`,
		`(?i)on\w+\s*=`,
		`(?i)<iframe`,
		`(?i)<object`,
		`(?i)<embed`,
		`(?i)<svg[^>]*onload`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &XSSMiddleware{patterns: compiled}
}

// Handler returns the middleware handler
func (xss *XSSMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check query parameters
			for _, values := range r.URL.Query() {
				for _, value := range values {
					if xss.isSuspicious(value) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{
							"error":   "invalid_request",
							"message": "Request contains potentially malicious content",
						})
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSuspicious checks if input matches XSS patterns
func (xss *XSSMiddleware) isSuspicious(input string) bool {
	for _, pattern := range xss.patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// itoa converts int to string (avoiding strconv import for small numbers)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// ChainMiddleware chains multiple middleware together
func ChainMiddleware(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
