package auth

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"sync"
)

// Manager provides simple in-memory token management.
type Manager struct {
	mu     sync.Mutex
	tokens map[string]string // token -> userID
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{tokens: make(map[string]string)}
}

// GenerateToken registers a new token for the given user ID and returns it.
func (m *Manager) GenerateToken(userID string) (string, error) {
	token, err := randomString(32)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = userID
	return token, nil
}

// ValidateToken checks if the token exists and returns the associated user ID.
func (m *Manager) ValidateToken(token string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	userID, ok := m.tokens[token]
	return userID, ok
}

// RevokeToken removes the token from the manager.
func (m *Manager) RevokeToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, token)
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
