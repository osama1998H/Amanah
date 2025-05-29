package services

import (
	"testing"

	"amanah/libs/auth"
	"amanah/services/authentication/models"
	"amanah/services/authentication/repositories"
)

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
