package services

import (
	"sync"
	"testing"

	"amanah/libs/auth"
	"amanah/services/authentication/models"
	"amanah/services/authentication/repositories"
)

// Test helpers
func setupTestService() (*AuthService, *repositories.InMemoryUserRepo, *auth.Manager) {
	repo := repositories.NewInMemoryUserRepo()
	tokenManager := auth.NewManager()
	service := NewAuthService(repo, tokenManager)
	return service, repo, tokenManager
}

func createTestUser(repo *repositories.InMemoryUserRepo, id, username, password string) {
	repo.AddUser(models.User{
		ID:       id,
		Username: username,
		Password: password,
	})
}

// ===== Login Tests =====

func TestAuthServiceLogin(t *testing.T) {
	repo := repositories.NewInMemoryUserRepo()
	repo.AddUser(models.User{ID: "1", Username: "user", Password: "secret"})

	svc := NewAuthService(repo, auth.NewManager())

	token, err := svc.Login("user", "secret")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatalf("expected token, got empty string")
	}

	if _, err := svc.Login("user", "wrong"); err == nil {
		t.Fatalf("expected error for wrong password")
	}
}

func TestLogin_ValidCredentials(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "password123")

	token, err := svc.Login("testuser", "password123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "correctpassword")

	_, err := svc.Login("testuser", "wrongpassword")
	if err == nil {
		t.Error("expected error for invalid password")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Login("nonexistent", "password")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestLogin_EmptyUsername(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	_, err := svc.Login("", "password")
	if err == nil {
		t.Error("expected error for empty username")
	}
}

func TestLogin_EmptyPassword(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	_, err := svc.Login("testuser", "")
	if err == nil {
		t.Error("expected error for empty password")
	}
}

func TestLogin_BothEmpty(t *testing.T) {
	svc, _, _ := setupTestService()

	_, err := svc.Login("", "")
	if err == nil {
		t.Error("expected error for empty credentials")
	}
}

func TestLogin_CaseSensitiveUsername(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "TestUser", "password")

	// Should fail - username is case sensitive
	_, err := svc.Login("testuser", "password")
	if err == nil {
		t.Error("expected error for case-sensitive username")
	}

	// Should succeed with correct case
	token, err := svc.Login("TestUser", "password")
	if err != nil {
		t.Fatalf("expected no error with correct username case, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_CaseSensitivePassword(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "Password123")

	// Should fail - password is case sensitive
	_, err := svc.Login("testuser", "password123")
	if err == nil {
		t.Error("expected error for case-sensitive password")
	}

	// Should succeed with correct case
	_, err = svc.Login("testuser", "Password123")
	if err != nil {
		t.Errorf("expected no error with correct password case, got %v", err)
	}
}

func TestLogin_SpecialCharactersInPassword(t *testing.T) {
	svc, repo, _ := setupTestService()
	specialPassword := "P@$$w0rd!#%^&*()"
	createTestUser(repo, "user1", "testuser", specialPassword)

	token, err := svc.Login("testuser", specialPassword)
	if err != nil {
		t.Fatalf("expected no error with special characters in password, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_WhitespaceInCredentials(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "test user", "pass word")

	token, err := svc.Login("test user", "pass word")
	if err != nil {
		t.Fatalf("expected no error with whitespace in credentials, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_UniqueTokensGenerated(t *testing.T) {
	svc, repo, _ := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := svc.Login("testuser", "password")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tokens[token] {
			t.Errorf("duplicate token generated: %s", token)
		}
		tokens[token] = true
	}
}

func TestLogin_MultipleUsers(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "alice", "alice123")
	createTestUser(repo, "user2", "bob", "bob456")
	createTestUser(repo, "user3", "charlie", "charlie789")

	tests := []struct {
		username string
		password string
	}{
		{"alice", "alice123"},
		{"bob", "bob456"},
		{"charlie", "charlie789"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			token, err := svc.Login(tt.username, tt.password)
			if err != nil {
				t.Fatalf("expected no error for %s, got %v", tt.username, err)
			}
			if token == "" {
				t.Error("expected token to be non-empty")
			}

			// Verify token is valid
			userID, ok := tokenMgr.ValidateToken(token)
			if !ok {
				t.Error("expected token to be valid")
			}
			if userID == "" {
				t.Error("expected userID to be non-empty")
			}
		})
	}
}

// ===== Token Management Tests =====

func TestTokenValidation_ValidToken(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	token, _ := svc.Login("testuser", "password")

	userID, ok := tokenMgr.ValidateToken(token)
	if !ok {
		t.Error("expected token to be valid")
	}
	if userID != "user1" {
		t.Errorf("expected userID 'user1', got %s", userID)
	}
}

func TestTokenValidation_InvalidToken(t *testing.T) {
	_, _, tokenMgr := setupTestService()

	_, ok := tokenMgr.ValidateToken("invalidtoken123")
	if ok {
		t.Error("expected invalid token to fail validation")
	}
}

func TestTokenValidation_EmptyToken(t *testing.T) {
	_, _, tokenMgr := setupTestService()

	_, ok := tokenMgr.ValidateToken("")
	if ok {
		t.Error("expected empty token to fail validation")
	}
}

func TestTokenRevocation(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	token, _ := svc.Login("testuser", "password")

	// Token should be valid initially
	if _, ok := tokenMgr.ValidateToken(token); !ok {
		t.Error("expected token to be valid before revocation")
	}

	// Revoke the token
	tokenMgr.RevokeToken(token)

	// Token should be invalid after revocation
	if _, ok := tokenMgr.ValidateToken(token); ok {
		t.Error("expected token to be invalid after revocation")
	}
}

func TestTokenRevocation_NonExistentToken(t *testing.T) {
	_, _, tokenMgr := setupTestService()

	// Should not panic when revoking non-existent token
	tokenMgr.RevokeToken("nonexistent")
}

func TestTokenRevocation_DoesNotAffectOtherTokens(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	token1, _ := svc.Login("testuser", "password")
	token2, _ := svc.Login("testuser", "password")

	// Both tokens should be valid
	if _, ok := tokenMgr.ValidateToken(token1); !ok {
		t.Error("expected token1 to be valid")
	}
	if _, ok := tokenMgr.ValidateToken(token2); !ok {
		t.Error("expected token2 to be valid")
	}

	// Revoke only token1
	tokenMgr.RevokeToken(token1)

	// token1 should be invalid, token2 should still be valid
	if _, ok := tokenMgr.ValidateToken(token1); ok {
		t.Error("expected token1 to be invalid after revocation")
	}
	if _, ok := tokenMgr.ValidateToken(token2); !ok {
		t.Error("expected token2 to still be valid after token1 revocation")
	}
}

// ===== Concurrent Access Tests =====

func TestConcurrentLogins(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	var wg sync.WaitGroup
	tokens := make(chan string, 100)
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := svc.Login("testuser", "password")
			if err != nil {
				errors <- err
				return
			}
			tokens <- token
		}()
	}

	wg.Wait()
	close(tokens)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("unexpected error during concurrent login: %v", err)
	}

	// Verify all tokens are unique and valid
	tokenSet := make(map[string]bool)
	for token := range tokens {
		if tokenSet[token] {
			t.Error("duplicate token generated")
		}
		tokenSet[token] = true

		if _, ok := tokenMgr.ValidateToken(token); !ok {
			t.Error("generated token is not valid")
		}
	}

	if len(tokenSet) != 100 {
		t.Errorf("expected 100 unique tokens, got %d", len(tokenSet))
	}
}

func TestConcurrentTokenValidation(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	token, _ := svc.Login("testuser", "password")

	var wg sync.WaitGroup
	results := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := tokenMgr.ValidateToken(token)
			results <- ok
		}()
	}

	wg.Wait()
	close(results)

	// All validations should succeed
	for ok := range results {
		if !ok {
			t.Error("token validation failed during concurrent access")
		}
	}
}

func TestConcurrentTokenRevocation(t *testing.T) {
	svc, repo, tokenMgr := setupTestService()
	createTestUser(repo, "user1", "testuser", "password")

	var wg sync.WaitGroup

	// Generate tokens
	tokens := make([]string, 50)
	for i := 0; i < 50; i++ {
		token, _ := svc.Login("testuser", "password")
		tokens[i] = token
	}

	// Concurrently revoke and validate
	for i := 0; i < 50; i++ {
		wg.Add(2)
		idx := i
		go func() {
			defer wg.Done()
			tokenMgr.RevokeToken(tokens[idx])
		}()
		go func() {
			defer wg.Done()
			tokenMgr.ValidateToken(tokens[idx])
		}()
	}

	wg.Wait()
	// Test should complete without deadlock or panic
}

// ===== Edge Cases =====

func TestLogin_VeryLongUsername(t *testing.T) {
	svc, repo, _ := setupTestService()
	longUsername := make([]byte, 1000)
	for i := range longUsername {
		longUsername[i] = 'a'
	}
	createTestUser(repo, "user1", string(longUsername), "password")

	token, err := svc.Login(string(longUsername), "password")
	if err != nil {
		t.Fatalf("expected no error with long username, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_VeryLongPassword(t *testing.T) {
	svc, repo, _ := setupTestService()
	longPassword := make([]byte, 1000)
	for i := range longPassword {
		longPassword[i] = 'p'
	}
	createTestUser(repo, "user1", "testuser", string(longPassword))

	token, err := svc.Login("testuser", string(longPassword))
	if err != nil {
		t.Fatalf("expected no error with long password, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_UnicodeCredentials(t *testing.T) {
	svc, repo, _ := setupTestService()
	unicodeUsername := "用户名"
	unicodePassword := "密码🔐"
	createTestUser(repo, "user1", unicodeUsername, unicodePassword)

	token, err := svc.Login(unicodeUsername, unicodePassword)
	if err != nil {
		t.Fatalf("expected no error with unicode credentials, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

// ===== Service Creation Tests =====

func TestNewAuthService(t *testing.T) {
	repo := repositories.NewInMemoryUserRepo()
	tokenMgr := auth.NewManager()

	svc := NewAuthService(repo, tokenMgr)
	if svc == nil {
		t.Error("expected service to be created")
	}
}

// ===== Table Driven Tests =====

func TestLogin_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		setupUser bool
		wantErr   bool
	}{
		{
			name:      "valid credentials",
			username:  "testuser",
			password:  "password123",
			setupUser: true,
			wantErr:   false,
		},
		{
			name:      "wrong password",
			username:  "testuser",
			password:  "wrongpassword",
			setupUser: true,
			wantErr:   true,
		},
		{
			name:      "user not found",
			username:  "nonexistent",
			password:  "password",
			setupUser: false,
			wantErr:   true,
		},
		{
			name:      "empty username",
			username:  "",
			password:  "password",
			setupUser: false,
			wantErr:   true,
		},
		{
			name:      "empty password",
			username:  "testuser",
			password:  "",
			setupUser: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := setupTestService()
			if tt.setupUser {
				createTestUser(repo, "user1", "testuser", "password123")
			}

			token, err := svc.Login(tt.username, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got token: %s", token)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if token == "" {
					t.Error("expected non-empty token")
				}
			}
		})
	}
}
