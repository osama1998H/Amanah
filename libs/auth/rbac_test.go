package auth

import (
	"testing"
)

func TestRBACManagerDefaultRoles(t *testing.T) {
	rbac := NewRBACManager()

	// Check default roles exist
	defaultRoles := []string{"user", "merchant", "operator", "admin", "service", "superadmin"}
	for _, roleName := range defaultRoles {
		role, err := rbac.GetRole(roleName)
		if err != nil {
			t.Errorf("Expected role %s to exist, got error: %v", roleName, err)
		}
		if role.Name != roleName {
			t.Errorf("Expected role name %s, got %s", roleName, role.Name)
		}
	}
}

func TestRBACManagerHasPermission(t *testing.T) {
	rbac := NewRBACManager()

	tests := []struct {
		role       string
		permission Permission
		expected   bool
	}{
		{"user", PermTransactionRead, true},
		{"user", PermTransactionCreate, true},
		{"user", PermAdminAccess, false},
		{"merchant", PermTransactionRefund, true},
		{"merchant", PermReportView, true},
		{"operator", PermAccountFreeze, true},
		{"operator", PermTransactionRefund, false},
		{"admin", PermAdminAccess, true},
		{"admin", PermAdminUserManage, true},
		{"superadmin", PermAdminAccess, true}, // Superadmin bypasses all
		{"superadmin", "any:permission", true},
	}

	for _, tt := range tests {
		t.Run(tt.role+":"+string(tt.permission), func(t *testing.T) {
			result := rbac.HasPermission(tt.role, tt.permission)
			if result != tt.expected {
				t.Errorf("HasPermission(%s, %s) = %v, expected %v", tt.role, tt.permission, result, tt.expected)
			}
		})
	}
}

func TestRBACManagerInheritance(t *testing.T) {
	rbac := NewRBACManager()

	// Admin inherits from operator and merchant
	// So admin should have permissions from both

	// Check operator permissions
	if !rbac.HasPermission("admin", PermAccountFreeze) {
		t.Error("Admin should inherit PermAccountFreeze from operator")
	}

	// Check merchant permissions
	if !rbac.HasPermission("admin", PermTransactionRefund) {
		t.Error("Admin should inherit PermTransactionRefund from merchant")
	}

	// Check user permissions (inherited through merchant and operator)
	if !rbac.HasPermission("admin", PermTransactionRead) {
		t.Error("Admin should inherit PermTransactionRead through inheritance chain")
	}
}

func TestRBACManagerGetAllPermissions(t *testing.T) {
	rbac := NewRBACManager()

	perms, err := rbac.GetAllPermissions("admin")
	if err != nil {
		t.Fatalf("Failed to get permissions: %v", err)
	}

	if len(perms) == 0 {
		t.Error("Admin should have permissions")
	}

	// Check that admin has its own permissions
	hasAdminAccess := false
	for _, p := range perms {
		if p == PermAdminAccess {
			hasAdminAccess = true
			break
		}
	}

	if !hasAdminAccess {
		t.Error("Admin permissions should include PermAdminAccess")
	}
}

func TestRBACManagerCheckAccess(t *testing.T) {
	rbac := NewRBACManager()

	// User with multiple roles
	userRoles := []string{"user", "merchant"}

	// Should have access (merchant has this)
	if err := rbac.CheckAccess(userRoles, PermTransactionRefund); err != nil {
		t.Errorf("Expected access to PermTransactionRefund, got error: %v", err)
	}

	// Should not have access
	if err := rbac.CheckAccess(userRoles, PermAdminAccess); err != ErrAccessDenied {
		t.Errorf("Expected ErrAccessDenied for PermAdminAccess, got: %v", err)
	}
}

func TestRBACManagerCheckAnyAccess(t *testing.T) {
	rbac := NewRBACManager()

	userRoles := []string{"user"}

	// Should have access to at least one
	err := rbac.CheckAnyAccess(userRoles, PermAdminAccess, PermTransactionRead)
	if err != nil {
		t.Errorf("Expected access to at least one permission, got error: %v", err)
	}

	// Should not have access to any
	err = rbac.CheckAnyAccess(userRoles, PermAdminAccess, PermAdminUserManage)
	if err != ErrAccessDenied {
		t.Errorf("Expected ErrAccessDenied, got: %v", err)
	}
}

func TestRBACManagerAddRole(t *testing.T) {
	rbac := NewRBACManager()

	customRole := &Role{
		Name:        "custom",
		Description: "Custom role for testing",
		Permissions: []Permission{PermTransactionRead, PermAPIRead},
	}

	err := rbac.AddRole(customRole)
	if err != nil {
		t.Fatalf("Failed to add role: %v", err)
	}

	role, err := rbac.GetRole("custom")
	if err != nil {
		t.Fatalf("Failed to get added role: %v", err)
	}

	if role.Description != "Custom role for testing" {
		t.Errorf("Expected description 'Custom role for testing', got '%s'", role.Description)
	}

	if !rbac.HasPermission("custom", PermTransactionRead) {
		t.Error("Custom role should have PermTransactionRead")
	}
}

func TestRBACManagerRemoveRole(t *testing.T) {
	rbac := NewRBACManager()

	// Add a role first
	rbac.AddRole(&Role{Name: "temp", Permissions: []Permission{PermAPIRead}})

	// Remove it
	err := rbac.RemoveRole("temp")
	if err != nil {
		t.Fatalf("Failed to remove role: %v", err)
	}

	// Should not exist anymore
	_, err = rbac.GetRole("temp")
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got: %v", err)
	}

	// Remove non-existent role
	err = rbac.RemoveRole("nonexistent")
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got: %v", err)
	}
}

func TestRBACManagerAddPermissionToRole(t *testing.T) {
	rbac := NewRBACManager()

	// Add permission to existing role
	err := rbac.AddPermissionToRole("user", PermAdminAccess)
	if err != nil {
		t.Fatalf("Failed to add permission: %v", err)
	}

	if !rbac.HasPermission("user", PermAdminAccess) {
		t.Error("User should now have PermAdminAccess")
	}

	// Adding same permission again should not error
	err = rbac.AddPermissionToRole("user", PermAdminAccess)
	if err != nil {
		t.Errorf("Adding duplicate permission should not error: %v", err)
	}

	// Add to non-existent role
	err = rbac.AddPermissionToRole("nonexistent", PermAPIRead)
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got: %v", err)
	}
}

func TestRBACManagerRemovePermissionFromRole(t *testing.T) {
	rbac := NewRBACManager()

	// Remove existing permission
	err := rbac.RemovePermissionFromRole("user", PermTransactionRead)
	if err != nil {
		t.Fatalf("Failed to remove permission: %v", err)
	}

	if rbac.HasPermission("user", PermTransactionRead) {
		t.Error("User should no longer have PermTransactionRead")
	}

	// Remove non-existent permission
	err = rbac.RemovePermissionFromRole("user", PermAdminAccess)
	if err != ErrPermissionNotFound {
		t.Errorf("Expected ErrPermissionNotFound, got: %v", err)
	}

	// Remove from non-existent role
	err = rbac.RemovePermissionFromRole("nonexistent", PermAPIRead)
	if err != ErrRoleNotFound {
		t.Errorf("Expected ErrRoleNotFound, got: %v", err)
	}
}

func TestRBACManagerListRoles(t *testing.T) {
	rbac := NewRBACManager()

	roles := rbac.ListRoles()
	if len(roles) < 6 { // At least the 6 default roles
		t.Errorf("Expected at least 6 roles, got %d", len(roles))
	}

	roleNames := make(map[string]bool)
	for _, role := range roles {
		roleNames[role.Name] = true
	}

	expectedRoles := []string{"user", "merchant", "operator", "admin", "service", "superadmin"}
	for _, expected := range expectedRoles {
		if !roleNames[expected] {
			t.Errorf("Expected role %s in list", expected)
		}
	}
}

func TestRBACManagerHasAnyPermission(t *testing.T) {
	rbac := NewRBACManager()

	// User has TransactionRead but not AdminAccess
	if !rbac.HasAnyPermission("user", PermAdminAccess, PermTransactionRead) {
		t.Error("User should have at least one of the permissions")
	}

	if rbac.HasAnyPermission("user", PermAdminAccess, PermAdminUserManage) {
		t.Error("User should not have any of the admin permissions")
	}
}

func TestRBACManagerHasAllPermissions(t *testing.T) {
	rbac := NewRBACManager()

	// User has both
	if !rbac.HasAllPermissions("user", PermTransactionRead, PermTransactionCreate) {
		t.Error("User should have all basic transaction permissions")
	}

	// User doesn't have admin permission
	if rbac.HasAllPermissions("user", PermTransactionRead, PermAdminAccess) {
		t.Error("User should not have all permissions when admin is included")
	}
}

func TestRBACManagerCircularInheritance(t *testing.T) {
	rbac := NewRBACManager()

	// Create roles with circular inheritance
	rbac.AddRole(&Role{
		Name:        "roleA",
		Permissions: []Permission{PermAPIRead},
		Inherits:    []string{"roleB"},
	})

	rbac.AddRole(&Role{
		Name:        "roleB",
		Permissions: []Permission{PermAPIWrite},
		Inherits:    []string{"roleA"},
	})

	// Should not cause infinite loop
	perms, err := rbac.GetAllPermissions("roleA")
	if err != nil {
		t.Fatalf("Should handle circular inheritance: %v", err)
	}

	// Should have both permissions
	hasRead := false
	hasWrite := false
	for _, p := range perms {
		if p == PermAPIRead {
			hasRead = true
		}
		if p == PermAPIWrite {
			hasWrite = true
		}
	}

	if !hasRead || !hasWrite {
		t.Error("Should have permissions from both roles in circular inheritance")
	}
}
