package repositories

import (
	"testing"

	"amanah/services/authentication/models"
)

func TestNewInMemoryUserRepo(t *testing.T) {
	repo := NewInMemoryUserRepo()
	if repo == nil {
		t.Error("expected repository to be created")
	}
	if repo.users == nil {
		t.Error("expected users map to be initialized")
	}
}

func TestAddUser_Success(t *testing.T) {
	repo := NewInMemoryUserRepo()

	user := models.User{
		ID:       "user1",
		Username: "testuser",
		Password: "password123",
	}
	repo.AddUser(user)

	// Verify user was added
	retrieved, ok := repo.GetByUsername("testuser")
	if !ok {
		t.Fatal("expected user to be found")
	}
	if retrieved.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, retrieved.ID)
	}
	if retrieved.Username != user.Username {
		t.Errorf("expected username %s, got %s", user.Username, retrieved.Username)
	}
	if retrieved.Password != user.Password {
		t.Errorf("expected password %s, got %s", user.Password, retrieved.Password)
	}
}

func TestAddUser_OverwritesSameUsername(t *testing.T) {
	repo := NewInMemoryUserRepo()

	user1 := models.User{ID: "user1", Username: "testuser", Password: "pass1"}
	user2 := models.User{ID: "user2", Username: "testuser", Password: "pass2"}

	repo.AddUser(user1)
	repo.AddUser(user2)

	retrieved, ok := repo.GetByUsername("testuser")
	if !ok {
		t.Fatal("expected user to be found")
	}

	// Should be the second user since it overwrites
	if retrieved.ID != "user2" {
		t.Errorf("expected ID user2, got %s", retrieved.ID)
	}
	if retrieved.Password != "pass2" {
		t.Errorf("expected password pass2, got %s", retrieved.Password)
	}
}

func TestGetByUsername_Found(t *testing.T) {
	repo := NewInMemoryUserRepo()
	user := models.User{ID: "user1", Username: "testuser", Password: "pass"}
	repo.AddUser(user)

	retrieved, ok := repo.GetByUsername("testuser")
	if !ok {
		t.Error("expected user to be found")
	}
	if retrieved.ID != "user1" {
		t.Errorf("expected ID user1, got %s", retrieved.ID)
	}
}

func TestGetByUsername_NotFound(t *testing.T) {
	repo := NewInMemoryUserRepo()

	_, ok := repo.GetByUsername("nonexistent")
	if ok {
		t.Error("expected user to not be found")
	}
}

func TestGetByUsername_EmptyUsername(t *testing.T) {
	repo := NewInMemoryUserRepo()

	_, ok := repo.GetByUsername("")
	if ok {
		t.Error("expected empty username to not be found")
	}
}

func TestGetByUsername_CaseSensitive(t *testing.T) {
	repo := NewInMemoryUserRepo()
	user := models.User{ID: "user1", Username: "TestUser", Password: "pass"}
	repo.AddUser(user)

	// Exact case should work
	_, ok := repo.GetByUsername("TestUser")
	if !ok {
		t.Error("expected user to be found with exact case")
	}

	// Wrong case should not work
	_, ok = repo.GetByUsername("testuser")
	if ok {
		t.Error("expected user to not be found with different case")
	}

	_, ok = repo.GetByUsername("TESTUSER")
	if ok {
		t.Error("expected user to not be found with uppercase")
	}
}

func TestAddUser_EmptyFields(t *testing.T) {
	repo := NewInMemoryUserRepo()

	// Add user with empty fields
	user := models.User{ID: "", Username: "", Password: ""}
	repo.AddUser(user)

	// Should be findable by empty username
	_, ok := repo.GetByUsername("")
	if !ok {
		t.Error("expected empty username user to be found")
	}
}

func TestAddUser_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"email format", "user@example.com", "pass123"},
		{"special chars", "user!@#$%", "p@$$w0rd!"},
		{"unicode", "用户", "密码"},
		{"whitespace", "user name", "pass word"},
		{"newlines", "user\nname", "pass\nword"},
		{"tabs", "user\tname", "pass\tword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewInMemoryUserRepo()
			user := models.User{
				ID:       "user1",
				Username: tt.username,
				Password: tt.password,
			}
			repo.AddUser(user)

			retrieved, ok := repo.GetByUsername(tt.username)
			if !ok {
				t.Fatal("expected user to be found")
			}
			if retrieved.Password != tt.password {
				t.Errorf("expected password %s, got %s", tt.password, retrieved.Password)
			}
		})
	}
}

func TestAddUser_LongStrings(t *testing.T) {
	repo := NewInMemoryUserRepo()

	longStr := make([]byte, 10000)
	for i := range longStr {
		longStr[i] = 'a'
	}

	user := models.User{
		ID:       string(longStr),
		Username: string(longStr),
		Password: string(longStr),
	}
	repo.AddUser(user)

	retrieved, ok := repo.GetByUsername(string(longStr))
	if !ok {
		t.Fatal("expected user with long username to be found")
	}
	if len(retrieved.Password) != 10000 {
		t.Errorf("expected password length 10000, got %d", len(retrieved.Password))
	}
}

func TestMultipleUsers(t *testing.T) {
	repo := NewInMemoryUserRepo()

	users := []models.User{
		{ID: "1", Username: "alice", Password: "pass1"},
		{ID: "2", Username: "bob", Password: "pass2"},
		{ID: "3", Username: "charlie", Password: "pass3"},
		{ID: "4", Username: "david", Password: "pass4"},
		{ID: "5", Username: "eve", Password: "pass5"},
	}

	for _, u := range users {
		repo.AddUser(u)
	}

	// Verify all users are accessible
	for _, expected := range users {
		retrieved, ok := repo.GetByUsername(expected.Username)
		if !ok {
			t.Errorf("expected user %s to be found", expected.Username)
			continue
		}
		if retrieved.ID != expected.ID {
			t.Errorf("user %s: expected ID %s, got %s", expected.Username, expected.ID, retrieved.ID)
		}
		if retrieved.Password != expected.Password {
			t.Errorf("user %s: expected password %s, got %s", expected.Username, expected.Password, retrieved.Password)
		}
	}
}

// Note: InMemoryUserRepo is not designed for concurrent access.
// For concurrent operations, use a repository with proper synchronization
// like the account service's InMemoryRepository which uses sync.RWMutex.

// ===== User Model Tests =====

func TestUserModel_Fields(t *testing.T) {
	user := models.User{
		ID:       "test-id",
		Username: "test-username",
		Password: "test-password",
	}

	if user.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", user.ID)
	}
	if user.Username != "test-username" {
		t.Errorf("expected Username 'test-username', got %s", user.Username)
	}
	if user.Password != "test-password" {
		t.Errorf("expected Password 'test-password', got %s", user.Password)
	}
}

func TestUserModel_ZeroValue(t *testing.T) {
	var user models.User

	if user.ID != "" {
		t.Errorf("expected empty ID, got %s", user.ID)
	}
	if user.Username != "" {
		t.Errorf("expected empty Username, got %s", user.Username)
	}
	if user.Password != "" {
		t.Errorf("expected empty Password, got %s", user.Password)
	}
}

func TestUserModel_Copy(t *testing.T) {
	original := models.User{
		ID:       "original-id",
		Username: "original-user",
		Password: "original-pass",
	}

	copy := original
	copy.ID = "modified-id"
	copy.Username = "modified-user"
	copy.Password = "modified-pass"

	// Original should be unchanged
	if original.ID != "original-id" {
		t.Error("original ID was modified")
	}
	if original.Username != "original-user" {
		t.Error("original Username was modified")
	}
	if original.Password != "original-pass" {
		t.Error("original Password was modified")
	}

	// Copy should have new values
	if copy.ID != "modified-id" {
		t.Error("copy ID was not modified")
	}
}
