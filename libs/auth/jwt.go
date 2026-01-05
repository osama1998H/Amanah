package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JWT Errors
var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrMissingClaims    = errors.New("missing required claims")
)

// TokenType represents the type of JWT token
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims represents JWT claims
type Claims struct {
	// Standard claims
	Subject   string    `json:"sub"`           // User ID
	Issuer    string    `json:"iss,omitempty"` // Token issuer
	Audience  string    `json:"aud,omitempty"` // Token audience
	ExpiresAt int64     `json:"exp"`           // Expiration time
	IssuedAt  int64     `json:"iat"`           // Issued at time
	NotBefore int64     `json:"nbf,omitempty"` // Not valid before
	TokenID   string    `json:"jti,omitempty"` // Unique token ID
	TokenType TokenType `json:"type"`          // Token type (access/refresh)

	// Custom claims
	Email    string   `json:"email,omitempty"`
	Name     string   `json:"name,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	TenantID string   `json:"tenant_id,omitempty"`
	Scope    []string `json:"scope,omitempty"`
}

// Valid checks if the claims are valid
func (c *Claims) Valid() error {
	if c.Subject == "" {
		return ErrMissingClaims
	}

	now := time.Now().Unix()

	if c.ExpiresAt > 0 && now > c.ExpiresAt {
		return ErrTokenExpired
	}

	if c.NotBefore > 0 && now < c.NotBefore {
		return ErrInvalidToken
	}

	return nil
}

// HasRole checks if the claims include a specific role
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the claims include any of the specified roles
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// HasScope checks if the claims include a specific scope
func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scope {
		if s == scope {
			return true
		}
	}
	return false
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret            string
	Issuer            string
	Audience          string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		Secret:            "change-me-in-production",
		Issuer:            "amanah",
		Audience:          "amanah-api",
		AccessExpiration:  15 * time.Minute,
		RefreshExpiration: 7 * 24 * time.Hour,
	}
}

// JWTManager handles JWT token operations
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig) *JWTManager {
	if config == nil {
		config = DefaultJWTConfig()
	}
	return &JWTManager{config: config}
}

// header represents JWT header
type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// GenerateAccessToken generates a new access token
func (m *JWTManager) GenerateAccessToken(userID, email, name string, roles []string) (string, error) {
	claims := &Claims{
		Subject:   userID,
		Issuer:    m.config.Issuer,
		Audience:  m.config.Audience,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(m.config.AccessExpiration).Unix(),
		TokenType: AccessToken,
		Email:     email,
		Name:      name,
		Roles:     roles,
	}
	return m.generateToken(claims)
}

// GenerateRefreshToken generates a new refresh token
func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	tokenID, err := randomString(16)
	if err != nil {
		return "", err
	}

	claims := &Claims{
		Subject:   userID,
		Issuer:    m.config.Issuer,
		Audience:  m.config.Audience,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(m.config.RefreshExpiration).Unix(),
		TokenType: RefreshToken,
		TokenID:   tokenID,
	}
	return m.generateToken(claims)
}

// GenerateTokenPair generates both access and refresh tokens
func (m *JWTManager) GenerateTokenPair(userID, email, name string, roles []string) (accessToken, refreshToken string, err error) {
	accessToken, err = m.GenerateAccessToken(userID, email, name, roles)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = m.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	signature := m.sign(signingInput)
	providedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal([]byte(signature), providedSig) {
		return nil, ErrInvalidSignature
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	// Validate claims
	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return &claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token
func (m *JWTManager) RefreshAccessToken(refreshTokenString string, email, name string, roles []string) (string, error) {
	claims, err := m.ValidateToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	if claims.TokenType != RefreshToken {
		return "", ErrInvalidToken
	}

	return m.GenerateAccessToken(claims.Subject, email, name, roles)
}

// generateToken creates a signed JWT token
func (m *JWTManager) generateToken(claims *Claims) (string, error) {
	// Create header
	h := header{
		Algorithm: "HS256",
		Type:      "JWT",
	}

	headerJSON, err := json.Marshal(h)
	if err != nil {
		return "", err
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	signature := m.sign(signingInput)
	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureB64, nil
}

// sign creates HMAC-SHA256 signature
func (m *JWTManager) sign(input string) []byte {
	h := hmac.New(sha256.New, []byte(m.config.Secret))
	h.Write([]byte(input))
	return h.Sum(nil)
}

// TokenPair represents an access/refresh token pair
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"` // Seconds until access token expires
	ExpiresAt    time.Time `json:"expires_at"`
}

// GenerateTokenPairWithMetadata generates tokens with additional metadata
func (m *JWTManager) GenerateTokenPairWithMetadata(userID, email, name string, roles []string) (*TokenPair, error) {
	accessToken, refreshToken, err := m.GenerateTokenPair(userID, email, name, roles)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.config.AccessExpiration.Seconds()),
		ExpiresAt:    time.Now().Add(m.config.AccessExpiration),
	}, nil
}

// ExtractTokenFromHeader extracts a token from Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is empty")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return strings.TrimSpace(parts[1]), nil
}
