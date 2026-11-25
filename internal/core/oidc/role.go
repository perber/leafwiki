package oidc

import "github.com/perber/wiki/internal/core/auth"

// MapRole maps an OIDC "groups" claim (a flat list of group names) to a
// LeafWiki role, re-evaluated on every login per the fork's spec: membership
// in adminGroup wins over editorGroup if a user is in both; membership in
// neither falls back to the wiki's lowest role (RoleViewer). A caller should
// treat an empty adminGroup/editorGroup as "never matches" (an operator who
// hasn't configured one of the two groups just means every OIDC user falls
// through to viewer for that tier), which this function already does
// naturally since an empty claim value would never be sent by a real
// provider for an empty configured name.
func MapRole(groups []string, adminGroup, editorGroup string) string {
	for _, g := range groups {
		if adminGroup != "" && g == adminGroup {
			return auth.RoleAdmin
		}
	}
	for _, g := range groups {
		if editorGroup != "" && g == editorGroup {
			return auth.RoleEditor
		}
	}
	return auth.RoleViewer
}

func hasGroup(groups []string, group string) bool {
	for _, g := range groups {
		if group != "" && g == group {
			return true
		}
	}
	return false
}

// RoleSync selects how much of a user's role OIDC is allowed to control on
// each login. See cmd/leafwiki/flags_oidc.go's --oidc-role-sync for the
// operator-facing description.
type RoleSync string

const (
	// RoleSyncAll is the default: MapRole's full result wins outright, every
	// login — a change made inside LeafWiki (e.g. editor -> admin by another
	// admin) is overwritten on the user's next login.
	RoleSyncAll RoleSync = "all"
	// RoleSyncAdmin only ever promotes into or demotes out of admin based on
	// the groups claim; editor/viewer are managed entirely inside LeafWiki
	// once assigned and are never touched here again.
	RoleSyncAdmin RoleSync = "admin"
)

// ResolveSyncedRole decides the role to assign currentRole's owner on this
// login. currentRole is the role the user already has — for a brand-new
// account this is whatever role it was just created with (see MapRole),
// so "admin" mode naturally leaves that initial assignment alone unless the
// user isn't in adminGroup and the role happens to already be admin (which
// can't occur for a fresh account, only for a demotion case below).
func ResolveSyncedRole(mode RoleSync, groups []string, adminGroup, editorGroup, currentRole string) string {
	if mode != RoleSyncAdmin {
		return MapRole(groups, adminGroup, editorGroup)
	}
	if hasGroup(groups, adminGroup) {
		return auth.RoleAdmin
	}
	if currentRole == auth.RoleAdmin {
		// No longer in the admin group: demote. editor is the landing role
		// rather than viewer — losing admin shouldn't also strip the ability
		// to edit, and the operator can adjust it manually afterward.
		return auth.RoleEditor
	}
	return currentRole // editor/viewer: managed inside LeafWiki, left alone
}
