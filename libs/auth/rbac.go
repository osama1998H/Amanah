package auth

import (
	"errors"
	"sync"
)

// RBAC Errors
var (
	ErrAccessDenied       = errors.New("access denied")
	ErrRoleNotFound       = errors.New("role not found")
	ErrPermissionNotFound = errors.New("permission not found")
)

// Permission represents a permission
type Permission string

// Common permissions
const (
	// Transaction permissions
	PermTransactionCreate Permission = "transaction:create"
	PermTransactionRead   Permission = "transaction:read"
	PermTransactionUpdate Permission = "transaction:update"
	PermTransactionDelete Permission = "transaction:delete"
	PermTransactionRefund Permission = "transaction:refund"

	// Account permissions
	PermAccountCreate Permission = "account:create"
	PermAccountRead   Permission = "account:read"
	PermAccountUpdate Permission = "account:update"
	PermAccountDelete Permission = "account:delete"
	PermAccountFreeze Permission = "account:freeze"

	// User permissions
	PermUserCreate Permission = "user:create"
	PermUserRead   Permission = "user:read"
	PermUserUpdate Permission = "user:update"
	PermUserDelete Permission = "user:delete"

	// Admin permissions
	PermAdminAccess     Permission = "admin:access"
	PermAdminUserManage Permission = "admin:user_manage"
	PermAdminConfig     Permission = "admin:config"
	PermAdminAudit      Permission = "admin:audit"

	// API permissions
	PermAPIRead  Permission = "api:read"
	PermAPIWrite Permission = "api:write"

	// Report permissions
	PermReportView     Permission = "report:view"
	PermReportExport   Permission = "report:export"
	PermReportGenerate Permission = "report:generate"
)

// Role represents a role with associated permissions
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Inherits    []string     `json:"inherits,omitempty"` // Roles this role inherits from
}

// RBACManager manages role-based access control
type RBACManager struct {
	mu    sync.RWMutex
	roles map[string]*Role
}

// NewRBACManager creates a new RBAC manager with default roles
func NewRBACManager() *RBACManager {
	rbac := &RBACManager{
		roles: make(map[string]*Role),
	}

	// Initialize default roles
	rbac.initDefaultRoles()

	return rbac
}

// initDefaultRoles sets up the default role hierarchy
func (r *RBACManager) initDefaultRoles() {
	// User role - basic access
	r.roles["user"] = &Role{
		Name:        "user",
		Description: "Basic user with limited access",
		Permissions: []Permission{
			PermTransactionRead,
			PermTransactionCreate,
			PermAccountRead,
			PermUserRead,
		},
	}

	// Merchant role - can manage transactions
	r.roles["merchant"] = &Role{
		Name:        "merchant",
		Description: "Merchant with transaction management",
		Permissions: []Permission{
			PermTransactionCreate,
			PermTransactionRead,
			PermTransactionUpdate,
			PermTransactionRefund,
			PermAccountRead,
			PermAccountUpdate,
			PermReportView,
			PermReportExport,
		},
		Inherits: []string{"user"},
	}

	// Operator role - can manage accounts
	r.roles["operator"] = &Role{
		Name:        "operator",
		Description: "Operator with account management",
		Permissions: []Permission{
			PermAccountCreate,
			PermAccountRead,
			PermAccountUpdate,
			PermAccountFreeze,
			PermUserRead,
			PermUserUpdate,
			PermTransactionRead,
			PermReportView,
		},
		Inherits: []string{"user"},
	}

	// Admin role - full access
	r.roles["admin"] = &Role{
		Name:        "admin",
		Description: "Administrator with full access",
		Permissions: []Permission{
			PermAdminAccess,
			PermAdminUserManage,
			PermAdminConfig,
			PermAdminAudit,
			PermAccountDelete,
			PermUserCreate,
			PermUserDelete,
			PermTransactionDelete,
			PermReportGenerate,
		},
		Inherits: []string{"operator", "merchant"},
	}

	// Service role - for internal services
	r.roles["service"] = &Role{
		Name:        "service",
		Description: "Internal service account",
		Permissions: []Permission{
			PermAPIRead,
			PermAPIWrite,
			PermTransactionCreate,
			PermTransactionRead,
			PermTransactionUpdate,
			PermAccountRead,
			PermAccountUpdate,
		},
	}

	// Super admin role - unrestricted access
	r.roles["superadmin"] = &Role{
		Name:        "superadmin",
		Description: "Super administrator with unrestricted access",
		Permissions: []Permission{}, // Superadmin bypasses permission checks
		Inherits:    []string{"admin"},
	}
}

// AddRole adds a new role
func (r *RBACManager) AddRole(role *Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.roles[role.Name] = role
	return nil
}

// GetRole retrieves a role by name
func (r *RBACManager) GetRole(name string) (*Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[name]
	if !exists {
		return nil, ErrRoleNotFound
	}
	return role, nil
}

// GetAllPermissions returns all permissions for a role including inherited ones
func (r *RBACManager) GetAllPermissions(roleName string) ([]Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getAllPermissionsRecursive(roleName, make(map[string]bool))
}

// getAllPermissionsRecursive recursively gets all permissions including inherited
func (r *RBACManager) getAllPermissionsRecursive(roleName string, visited map[string]bool) ([]Permission, error) {
	if visited[roleName] {
		return nil, nil // Avoid circular dependencies
	}
	visited[roleName] = true

	role, exists := r.roles[roleName]
	if !exists {
		return nil, ErrRoleNotFound
	}

	permSet := make(map[Permission]bool)

	// Add direct permissions
	for _, perm := range role.Permissions {
		permSet[perm] = true
	}

	// Add inherited permissions
	for _, inheritedRole := range role.Inherits {
		inheritedPerms, err := r.getAllPermissionsRecursive(inheritedRole, visited)
		if err != nil {
			continue // Skip missing inherited roles
		}
		for _, perm := range inheritedPerms {
			permSet[perm] = true
		}
	}

	// Convert set to slice
	permissions := make([]Permission, 0, len(permSet))
	for perm := range permSet {
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// HasPermission checks if a role has a specific permission
func (r *RBACManager) HasPermission(roleName string, permission Permission) bool {
	// Superadmin bypasses all permission checks
	if roleName == "superadmin" {
		return true
	}

	permissions, err := r.GetAllPermissions(roleName)
	if err != nil {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

// HasAnyPermission checks if a role has any of the specified permissions
func (r *RBACManager) HasAnyPermission(roleName string, permissions ...Permission) bool {
	for _, perm := range permissions {
		if r.HasPermission(roleName, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if a role has all of the specified permissions
func (r *RBACManager) HasAllPermissions(roleName string, permissions ...Permission) bool {
	for _, perm := range permissions {
		if !r.HasPermission(roleName, perm) {
			return false
		}
	}
	return true
}

// CheckAccess verifies if any of the user's roles have the required permission
func (r *RBACManager) CheckAccess(userRoles []string, permission Permission) error {
	for _, role := range userRoles {
		if r.HasPermission(role, permission) {
			return nil
		}
	}
	return ErrAccessDenied
}

// CheckAnyAccess verifies if any of the user's roles have any of the required permissions
func (r *RBACManager) CheckAnyAccess(userRoles []string, permissions ...Permission) error {
	for _, role := range userRoles {
		if r.HasAnyPermission(role, permissions...) {
			return nil
		}
	}
	return ErrAccessDenied
}

// ListRoles returns all available roles
func (r *RBACManager) ListRoles() []*Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]*Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, role)
	}
	return roles
}

// RemoveRole removes a role
func (r *RBACManager) RemoveRole(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[name]; !exists {
		return ErrRoleNotFound
	}

	delete(r.roles, name)
	return nil
}

// AddPermissionToRole adds a permission to an existing role
func (r *RBACManager) AddPermissionToRole(roleName string, permission Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	role, exists := r.roles[roleName]
	if !exists {
		return ErrRoleNotFound
	}

	// Check if permission already exists
	for _, p := range role.Permissions {
		if p == permission {
			return nil // Already has this permission
		}
	}

	role.Permissions = append(role.Permissions, permission)
	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (r *RBACManager) RemovePermissionFromRole(roleName string, permission Permission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	role, exists := r.roles[roleName]
	if !exists {
		return ErrRoleNotFound
	}

	for i, p := range role.Permissions {
		if p == permission {
			role.Permissions = append(role.Permissions[:i], role.Permissions[i+1:]...)
			return nil
		}
	}

	return ErrPermissionNotFound
}
