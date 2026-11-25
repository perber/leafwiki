package oidc

import (
	"testing"

	"github.com/perber/wiki/internal/core/auth"
)

func TestMapRole(t *testing.T) {
	tests := []struct {
		name        string
		groups      []string
		adminGroup  string
		editorGroup string
		want        string
	}{
		{"admin group present", []string{"wiki-admins"}, "wiki-admins", "wiki-editeurs", "admin"},
		{"editor group present", []string{"wiki-editeurs"}, "wiki-admins", "wiki-editeurs", "editor"},
		{"neither matches", []string{"gpao-users"}, "wiki-admins", "wiki-editeurs", "viewer"},
		{"empty groups", nil, "wiki-admins", "wiki-editeurs", "viewer"},
		{"admin wins over editor when both present", []string{"wiki-editeurs", "wiki-admins"}, "wiki-admins", "wiki-editeurs", "admin"},
		{"unconfigured admin group never matches an empty claim value", []string{""}, "", "wiki-editeurs", "viewer"},
		{"multiple unrelated groups, one matches editor", []string{"gpao-users", "wiki-editeurs", "kanboard-users"}, "wiki-admins", "wiki-editeurs", "editor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapRole(tt.groups, tt.adminGroup, tt.editorGroup)
			if got != tt.want {
				t.Fatalf("MapRole(%v, %q, %q) = %q, want %q", tt.groups, tt.adminGroup, tt.editorGroup, got, tt.want)
			}
		})
	}
}

func TestResolveSyncedRole(t *testing.T) {
	const adminGroup = "wiki-admins"
	const editorGroup = "wiki-editeurs"

	tests := []struct {
		name        string
		mode        RoleSync
		groups      []string
		currentRole string
		want        string
	}{
		{"all mode: groups claim always wins, same as MapRole", RoleSyncAll, []string{editorGroup}, auth.RoleAdmin, auth.RoleEditor},
		{"all mode: empty groups demotes an existing admin to viewer", RoleSyncAll, nil, auth.RoleAdmin, auth.RoleViewer},
		{"admin mode: member of admin group is promoted regardless of current role", RoleSyncAdmin, []string{adminGroup}, auth.RoleViewer, auth.RoleAdmin},
		{"admin mode: removed from admin group demotes an existing admin to editor", RoleSyncAdmin, nil, auth.RoleAdmin, auth.RoleEditor},
		{"admin mode: an editor set by hand in LeafWiki is left alone", RoleSyncAdmin, nil, auth.RoleEditor, auth.RoleEditor},
		{"admin mode: a viewer not in any group is left alone", RoleSyncAdmin, []string{"gpao-users"}, auth.RoleViewer, auth.RoleViewer},
		{"admin mode: admin group wins even if also in editor group", RoleSyncAdmin, []string{editorGroup, adminGroup}, auth.RoleViewer, auth.RoleAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSyncedRole(tt.mode, tt.groups, adminGroup, editorGroup, tt.currentRole)
			if got != tt.want {
				t.Fatalf("ResolveSyncedRole(%q, %v, currentRole=%q) = %q, want %q", tt.mode, tt.groups, tt.currentRole, got, tt.want)
			}
		})
	}
}
