// Shared role-check predicate — used by RoleGuard (JSX) and the settings
// section registry (plain predicate, no JSX available there).
//
// An empty allowedRoles list means "any authenticated user", not "no one" —
// this matches the settings section registry's convention for sections
// (e.g. Account) that every logged-in user should see regardless of role.

export function hasRole(
  role: string | undefined,
  allowedRoles: string[],
): boolean {
  if (!role) return false
  if (allowedRoles.length === 0) return true
  return allowedRoles.includes(role)
}
