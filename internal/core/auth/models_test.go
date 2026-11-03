package auth

import "testing"

// TestHasAtLeastRole pins the role hierarchy viewer < editor < admin that
// RequireEditorOrAdmin (and, later, the MCP admin tools) build on: a role
// satisfies a minimum when it is the minimum itself or above it. Nil users
// and unknown roles (on either side) never satisfy.
func TestHasAtLeastRole(t *testing.T) {
	tests := []struct {
		name    string
		user    *User
		minRole string
		want    bool
	}{
		{name: "nil user with admin", user: nil, minRole: RoleAdmin, want: false},
		{name: "nil user with editor", user: nil, minRole: RoleEditor, want: false},
		{name: "nil user with viewer", user: nil, minRole: RoleViewer, want: false},

		{name: "admin has at least admin", user: &User{Role: RoleAdmin}, minRole: RoleAdmin, want: true},
		{name: "admin has at least editor", user: &User{Role: RoleAdmin}, minRole: RoleEditor, want: true},
		{name: "admin has at least viewer", user: &User{Role: RoleAdmin}, minRole: RoleViewer, want: true},

		{name: "editor lacks admin", user: &User{Role: RoleEditor}, minRole: RoleAdmin, want: false},
		{name: "editor has at least editor", user: &User{Role: RoleEditor}, minRole: RoleEditor, want: true},
		{name: "editor has at least viewer", user: &User{Role: RoleEditor}, minRole: RoleViewer, want: true},

		{name: "viewer lacks admin", user: &User{Role: RoleViewer}, minRole: RoleAdmin, want: false},
		{name: "viewer lacks editor", user: &User{Role: RoleViewer}, minRole: RoleEditor, want: false},
		{name: "viewer has at least viewer", user: &User{Role: RoleViewer}, minRole: RoleViewer, want: true},

		{name: "unknown user role has no minimum", user: &User{Role: ""}, minRole: RoleViewer, want: false},
		{name: "unknown user role does not match itself", user: &User{Role: "superuser"}, minRole: "superuser", want: false},

		{name: "admin does not satisfy an unknown minimum", user: &User{Role: RoleAdmin}, minRole: "superuser", want: false},
		{name: "viewer does not satisfy an unknown minimum", user: &User{Role: RoleViewer}, minRole: "bogus", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasAtLeastRole(tc.user, tc.minRole); got != tc.want {
				t.Errorf("HasAtLeastRole(%+v, %q) = %v, want %v", tc.user, tc.minRole, got, tc.want)
			}
		})
	}
}
