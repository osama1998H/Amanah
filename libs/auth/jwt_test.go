package auth

import (
	"testing"
	"time"
)

func TestJWTManagerGenerateAndValidate(t *testing.T) {
	config := &JWTConfig{
		Secret:            "test-secret-key-for-testing",
		Issuer:            "test-issuer",
		Audience:          "test-audience",
		AccessExpiration:  time.Hour,
		RefreshExpiration: 24 * time.Hour,
	}

	manager := NewJWTManager(config)

	// Generate access token
	token, err := manager.GenerateAccessToken("user123", "test@example.com", "Test User", []string{"user", "merchant"})
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	if token == "" {
		t.Fatal("Token should not be empty")
	}

	// Validate token
	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.Subject != "user123" {
		t.Errorf("Expected subject user123, got %s", claims.Subject)
	}

	if claims.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", claims.Email)
	}

	if claims.Name != "Test User" {
		t.Errorf("Expected name Test User, got %s", claims.Name)
	}

	if len(claims.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(claims.Roles))
	}
}

func TestJWTManagerTokenPair(t *testing.T) {
	manager := NewJWTManager(nil) // Use default config

	accessToken, refreshToken, err := manager.GenerateTokenPair("user456", "user@example.com", "User Name", []string{"user"})
	if err != nil {
		t.Fatalf("Failed to generate token pair: %v", err)
	}

	if accessToken == "" {
		t.Error("Access token should not be empty")
	}

	if refreshToken == "" {
		t.Error("Refresh token should not be empty")
	}

	// Validate access token
	accessClaims, err := manager.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("Failed to validate access token: %v", err)
	}

	if accessClaims.TokenType != AccessToken {
		t.Errorf("Expected AccessToken type, got %s", accessClaims.TokenType)
	}

	// Validate refresh token
	refreshClaims, err := manager.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("Failed to validate refresh token: %v", err)
	}

	if refreshClaims.TokenType != RefreshToken {
		t.Errorf("Expected RefreshToken type, got %s", refreshClaims.TokenType)
	}
}

func TestJWTManagerExpiredToken(t *testing.T) {
	config := &JWTConfig{
		Secret:           "test-secret",
		AccessExpiration: -time.Hour, // Already expired
	}

	manager := NewJWTManager(config)

	token, err := manager.GenerateAccessToken("user", "email", "name", nil)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	_, err = manager.ValidateToken(token)
	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}

func TestJWTManagerInvalidSignature(t *testing.T) {
	manager1 := NewJWTManager(&JWTConfig{Secret: "secret1"})
	manager2 := NewJWTManager(&JWTConfig{Secret: "secret2"})

	token, _ := manager1.GenerateAccessToken("user", "email", "name", nil)

	_, err := manager2.ValidateToken(token)
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestJWTManagerInvalidToken(t *testing.T) {
	manager := NewJWTManager(nil)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dots", "invalidtoken"},
		{"one dot", "invalid.token"},
		{"four dots", "a.b.c.d"},
		{"invalid base64", "invalid.!!!.token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ValidateToken(tt.token)
			if err == nil {
				t.Error("Expected error for invalid token")
			}
		})
	}
}

func TestJWTManagerRefreshAccessToken(t *testing.T) {
	config := &JWTConfig{
		Secret:            "test-secret",
		AccessExpiration:  time.Hour,
		RefreshExpiration: 24 * time.Hour,
	}

	manager := NewJWTManager(config)

	// Generate refresh token
	refreshToken, err := manager.GenerateRefreshToken("user123")
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	// Use refresh token to get new access token
	newAccessToken, err := manager.RefreshAccessToken(refreshToken, "new@email.com", "New Name", []string{"admin"})
	if err != nil {
		t.Fatalf("Failed to refresh access token: %v", err)
	}

	// Validate new access token
	claims, err := manager.ValidateToken(newAccessToken)
	if err != nil {
		t.Fatalf("Failed to validate new access token: %v", err)
	}

	if claims.Subject != "user123" {
		t.Errorf("Expected subject user123, got %s", claims.Subject)
	}

	if claims.Email != "new@email.com" {
		t.Errorf("Expected email new@email.com, got %s", claims.Email)
	}
}

func TestJWTManagerRefreshWithAccessToken(t *testing.T) {
	manager := NewJWTManager(nil)

	// Generate access token (not refresh)
	accessToken, _ := manager.GenerateAccessToken("user", "email", "name", nil)

	// Try to use access token as refresh token
	_, err := manager.RefreshAccessToken(accessToken, "email", "name", nil)
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken when using access token for refresh, got %v", err)
	}
}

func TestClaimsHasRole(t *testing.T) {
	claims := &Claims{
		Subject: "user",
		Roles:   []string{"admin", "user", "merchant"},
	}

	if !claims.HasRole("admin") {
		t.Error("Expected HasRole(admin) to return true")
	}

	if claims.HasRole("superadmin") {
		t.Error("Expected HasRole(superadmin) to return false")
	}
}

func TestClaimsHasAnyRole(t *testing.T) {
	claims := &Claims{
		Subject: "user",
		Roles:   []string{"user"},
	}

	if !claims.HasAnyRole("admin", "user") {
		t.Error("Expected HasAnyRole to return true")
	}

	if claims.HasAnyRole("admin", "superadmin") {
		t.Error("Expected HasAnyRole to return false")
	}
}

func TestClaimsHasScope(t *testing.T) {
	claims := &Claims{
		Subject: "user",
		Scope:   []string{"read", "write"},
	}

	if !claims.HasScope("read") {
		t.Error("Expected HasScope(read) to return true")
	}

	if claims.HasScope("delete") {
		t.Error("Expected HasScope(delete) to return false")
	}
}

func TestExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantErr   bool
	}{
		{"valid bearer", "Bearer abc123", "abc123", false},
		{"valid bearer with spaces", "Bearer   token  ", "token", false},
		{"lowercase bearer", "bearer abc123", "abc123", false},
		{"empty header", "", "", true},
		{"no bearer prefix", "abc123", "", true},
		{"only bearer", "Bearer", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ExtractTokenFromHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTokenFromHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
			if token != tt.wantToken {
				t.Errorf("ExtractTokenFromHeader() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestGenerateTokenPairWithMetadata(t *testing.T) {
	manager := NewJWTManager(nil)

	pair, err := manager.GenerateTokenPairWithMetadata("user", "email", "name", []string{"user"})
	if err != nil {
		t.Fatalf("Failed to generate token pair: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("Access token should not be empty")
	}

	if pair.RefreshToken == "" {
		t.Error("Refresh token should not be empty")
	}

	if pair.TokenType != "Bearer" {
		t.Errorf("Expected token type Bearer, got %s", pair.TokenType)
	}

	if pair.ExpiresIn <= 0 {
		t.Error("ExpiresIn should be positive")
	}

	if pair.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}
