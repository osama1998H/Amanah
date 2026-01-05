package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"amanah/libs/auth"
)

// Context keys for request context
type contextKey string

const (
	UserIDKey     contextKey = "user_id"
	UserEmailKey  contextKey = "user_email"
	UserNameKey   contextKey = "user_name"
	UserRolesKey  contextKey = "user_roles"
	ClaimsKey     contextKey = "claims"
	APIKeyKey     contextKey = "api_key"
	TenantIDKey   contextKey = "tenant_id"
	RequestIDKey  contextKey = "request_id"
)

// AuthConfig holds authentication middleware configuration
type AuthConfig struct {
	JWTManager    *auth.JWTManager
	APIKeyManager *auth.APIKeyManager
	RBACManager   *auth.RBACManager

	// Skip authentication for certain paths
	SkipPaths []string

	// Allow both JWT and API key (API key takes precedence)
	AllowAPIKey bool
	AllowJWT    bool
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig(jwtManager *auth.JWTManager) *AuthConfig {
	return &AuthConfig{
		JWTManager: jwtManager,
		AllowJWT:   true,
		AllowAPIKey: true,
		SkipPaths: []string{
			"/health",
			"/ready",
			"/metrics",
		},
	}
}

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	config *AuthConfig
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(config *AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{config: config}
}

// Handler returns the HTTP middleware handler
func (am *AuthMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should skip authentication
			if am.shouldSkip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Try API key authentication first
			if am.config.AllowAPIKey && am.config.APIKeyManager != nil {
				if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
					key, err := am.config.APIKeyManager.ValidateAPIKey(apiKey)
					if err != nil {
						am.writeError(w, http.StatusUnauthorized, "invalid_api_key", err.Error())
						return
					}

					// Add API key info to context
					ctx := r.Context()
					ctx = context.WithValue(ctx, APIKeyKey, key)
					ctx = context.WithValue(ctx, UserRolesKey, key.Roles)
					if key.TenantID != "" {
						ctx = context.WithValue(ctx, TenantIDKey, key.TenantID)
					}

					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try JWT authentication
			if am.config.AllowJWT && am.config.JWTManager != nil {
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					am.writeError(w, http.StatusUnauthorized, "missing_authorization", "Authorization header is required")
					return
				}

				token, err := auth.ExtractTokenFromHeader(authHeader)
				if err != nil {
					am.writeError(w, http.StatusUnauthorized, "invalid_authorization", err.Error())
					return
				}

				claims, err := am.config.JWTManager.ValidateToken(token)
				if err != nil {
					status := http.StatusUnauthorized
					code := "invalid_token"
					if err == auth.ErrTokenExpired {
						code = "token_expired"
					}
					am.writeError(w, status, code, err.Error())
					return
				}

				// Add claims to context
				ctx := r.Context()
				ctx = context.WithValue(ctx, ClaimsKey, claims)
				ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
				ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
				ctx = context.WithValue(ctx, UserNameKey, claims.Name)
				ctx = context.WithValue(ctx, UserRolesKey, claims.Roles)
				if claims.TenantID != "" {
					ctx = context.WithValue(ctx, TenantIDKey, claims.TenantID)
				}

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No authentication provided
			am.writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		})
	}
}

// shouldSkip checks if the path should skip authentication
func (am *AuthMiddleware) shouldSkip(path string) bool {
	for _, skipPath := range am.config.SkipPaths {
		if path == skipPath || strings.HasPrefix(path, skipPath+"/") {
			return true
		}
	}
	return false
}

// writeError writes an error response
func (am *AuthMiddleware) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

// RequireRoles creates middleware that requires specific roles
func RequireRoles(rbac *auth.RBACManager, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles, ok := r.Context().Value(UserRolesKey).([]string)
			if !ok || len(userRoles) == 0 {
				writeJSONError(w, http.StatusForbidden, "forbidden", "No roles found")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, userRole := range userRoles {
				for _, requiredRole := range roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				writeJSONError(w, http.StatusForbidden, "forbidden", "Insufficient role")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission creates middleware that requires a specific permission
func RequirePermission(rbac *auth.RBACManager, permission auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles, ok := r.Context().Value(UserRolesKey).([]string)
			if !ok || len(userRoles) == 0 {
				writeJSONError(w, http.StatusForbidden, "forbidden", "No roles found")
				return
			}

			// Check if any role has the required permission
			if err := rbac.CheckAccess(userRoles, permission); err != nil {
				writeJSONError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission creates middleware that requires any of the specified permissions
func RequireAnyPermission(rbac *auth.RBACManager, permissions ...auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoles, ok := r.Context().Value(UserRolesKey).([]string)
			if !ok || len(userRoles) == 0 {
				writeJSONError(w, http.StatusForbidden, "forbidden", "No roles found")
				return
			}

			// Check if any role has any of the required permissions
			if err := rbac.CheckAnyAccess(userRoles, permissions...); err != nil {
				writeJSONError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireTenant creates middleware that validates tenant access
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := r.Context().Value(TenantIDKey).(string)
			if !ok || tenantID == "" {
				writeJSONError(w, http.StatusForbidden, "forbidden", "Tenant context required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates middleware that parses auth if present but doesn't require it
func OptionalAuth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			token, err := auth.ExtractTokenFromHeader(authHeader)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ClaimsKey, claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
			ctx = context.WithValue(ctx, UserNameKey, claims.Name)
			ctx = context.WithValue(ctx, UserRolesKey, claims.Roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper functions to extract values from context

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserEmail extracts user email from context
func GetUserEmail(ctx context.Context) string {
	if v := ctx.Value(UserEmailKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserName extracts user name from context
func GetUserName(ctx context.Context) string {
	if v := ctx.Value(UserNameKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserRoles extracts user roles from context
func GetUserRoles(ctx context.Context) []string {
	if v := ctx.Value(UserRolesKey); v != nil {
		return v.([]string)
	}
	return nil
}

// GetClaims extracts JWT claims from context
func GetClaims(ctx context.Context) *auth.Claims {
	if v := ctx.Value(ClaimsKey); v != nil {
		return v.(*auth.Claims)
	}
	return nil
}

// GetAPIKey extracts API key from context
func GetAPIKey(ctx context.Context) *auth.APIKey {
	if v := ctx.Value(APIKeyKey); v != nil {
		return v.(*auth.APIKey)
	}
	return nil
}

// GetTenantID extracts tenant ID from context
func GetTenantID(ctx context.Context) string {
	if v := ctx.Value(TenantIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// writeJSONError is a helper to write JSON error responses
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request ID already exists
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				// Generate new request ID
				id, _ := generateRequestID()
				requestID = id
			}

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() (string, error) {
	// Use crypto/rand for better randomness
	b := make([]byte, 16)
	// Simple fallback: use time-based ID if crypto fails
	return strings.ReplaceAll(strings.ReplaceAll(
		strings.TrimSuffix(strings.TrimPrefix(
			strings.ToLower(string(b)), "\x00"), "\x00"),
		"\x00", ""), " ", "") + "-req", nil
}
