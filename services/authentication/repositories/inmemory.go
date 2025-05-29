package repositories

import "amanah/services/authentication/models"

// InMemoryUserRepo provides an in-memory user store.
type InMemoryUserRepo struct {
	users map[string]models.User // username -> user
}

// NewInMemoryUserRepo creates a repository with the provided initial users.
func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{users: make(map[string]models.User)}
}

// AddUser stores a user in the repo.
func (r *InMemoryUserRepo) AddUser(u models.User) {
	r.users[u.Username] = u
}

// GetByUsername returns a user by username and a boolean flag if found.
func (r *InMemoryUserRepo) GetByUsername(username string) (models.User, bool) {
	u, ok := r.users[username]
	return u, ok
}
