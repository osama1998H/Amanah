package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"amanah/libs/auth"
	"amanah/services/authentication/models"
	"amanah/services/authentication/repositories"
	"amanah/services/authentication/services"
)

// Test helpers
func setupTestHandler() (http.HandlerFunc, *repositories.InMemoryUserRepo) {
	repo := repositories.NewInMemoryUserRepo()
	tokenMgr := auth.NewManager()
	svc := services.NewAuthService(repo, tokenMgr)
	handler := LoginHandler(svc)
	return handler, repo
}

func createTestUser(repo *repositories.InMemoryUserRepo, id, username, password string) {
	repo.AddUser(models.User{
		ID:       id,
		Username: username,
		Password: password,
	})
}

// ===== LoginHandler Tests =====

func TestLoginHandler_ValidRequest(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"username":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Token == "" {
		t.Error("expected non-empty token in response")
	}
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler()

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestLoginHandler_MissingUsername(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing username, got %d", rr.Code)
	}
}

func TestLoginHandler_MissingPassword(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"username":"testuser"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing password, got %d", rr.Code)
	}
}

func TestLoginHandler_EmptyBody(t *testing.T) {
	handler, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d", rr.Code)
	}
}

func TestLoginHandler_WrongCredentials(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"username":"testuser","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong credentials, got %d", rr.Code)
	}
}

func TestLoginHandler_UserNotFound(t *testing.T) {
	handler, _ := setupTestHandler()

	body := `{"username":"nonexistent","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for non-existent user, got %d", rr.Code)
	}
}

func TestLoginHandler_ContentTypeJSON(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"username":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestLoginHandler_ResponseStructure(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	body := `{"username":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Error("expected 'token' field in response")
	}
}

func TestLoginHandler_NilBody(t *testing.T) {
	handler, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for nil body, got %d", rr.Code)
	}
}

func TestLoginHandler_ExtraFields(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	// Request with extra fields should still work
	body := `{"username":"testuser","password":"password123","extra":"field","another":123}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with extra fields, got %d", rr.Code)
	}
}

func TestLoginHandler_SpecialCharacters(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "test@user.com", "P@$$w0rd!#%")

	body := `{"username":"test@user.com","password":"P@$$w0rd!#%"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with special characters, got %d", rr.Code)
	}
}

func TestLoginHandler_Unicode(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "用户", "密码🔐")

	body := `{"username":"用户","password":"密码🔐"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with unicode, got %d", rr.Code)
	}
}

// ===== Table Driven Tests =====

func TestLoginHandler_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      bool
		body           string
		expectedStatus int
	}{
		{
			name:           "valid login",
			setupUser:      true,
			body:           `{"username":"testuser","password":"password123"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid json",
			setupUser:      false,
			body:           `{invalid}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			setupUser:      false,
			body:           ``,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "wrong password",
			setupUser:      true,
			body:           `{"username":"testuser","password":"wrong"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "user not found",
			setupUser:      false,
			body:           `{"username":"unknown","password":"pass"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing password",
			setupUser:      true,
			body:           `{"username":"testuser"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing username",
			setupUser:      true,
			body:           `{"password":"password123"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty object",
			setupUser:      false,
			body:           `{}`,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo := setupTestHandler()
			if tt.setupUser {
				createTestUser(repo, "user1", "testuser", "password123")
			}

			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

// ===== Request/Response Model Tests =====

func TestLoginRequest_Marshal(t *testing.T) {
	req := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded LoginRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Username != req.Username {
		t.Errorf("expected username %s, got %s", req.Username, decoded.Username)
	}
	if decoded.Password != req.Password {
		t.Errorf("expected password %s, got %s", req.Password, decoded.Password)
	}
}

func TestLoginResponse_Marshal(t *testing.T) {
	resp := LoginResponse{
		Token: "test-token-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded LoginResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Token != resp.Token {
		t.Errorf("expected token %s, got %s", resp.Token, decoded.Token)
	}
}

// ===== Edge Cases =====

func TestLoginHandler_LargePayload(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	// Create a large username
	largeUsername := make([]byte, 10000)
	for i := range largeUsername {
		largeUsername[i] = 'a'
	}

	body := map[string]string{
		"username": string(largeUsername),
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should return 401 since user doesn't exist with that username
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for unknown large username, got %d", rr.Code)
	}
}

func TestLoginHandler_WhitespaceCredentials(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "  test  ", "  pass  ")

	body := `{"username":"  test  ","password":"  pass  "}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should match exactly including whitespace
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with whitespace credentials, got %d", rr.Code)
	}
}

func TestLoginHandler_MultipleSuccessfulLogins(t *testing.T) {
	handler, repo := setupTestHandler()
	createTestUser(repo, "user1", "testuser", "password123")

	tokens := make(map[string]bool)
	for i := 0; i < 10; i++ {
		body := `{"username":"testuser","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("login %d: expected status 200, got %d", i, rr.Code)
		}

		var resp LoginResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("login %d: failed to decode response: %v", i, err)
		}

		if tokens[resp.Token] {
			t.Errorf("login %d: duplicate token generated", i)
		}
		tokens[resp.Token] = true
	}
}
