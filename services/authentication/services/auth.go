package services

import (
	"errors"

	"amanah/libs/auth"
	"amanah/services/authentication/repositories"
)

// AuthService handles user authentication and token generation.
type AuthService struct {
	repo   *repositories.InMemoryUserRepo
	tokens *auth.Manager
}

// NewAuthService creates a new AuthService.
func NewAuthService(repo *repositories.InMemoryUserRepo, tokens *auth.Manager) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

// Login validates credentials and returns a token.
func (s *AuthService) Login(username, password string) (string, error) {
	user, ok := s.repo.GetByUsername(username)
	if !ok || user.Password != password {
		return "", errors.New("invalid credentials")
	}
	return s.tokens.GenerateToken(user.ID)
}
