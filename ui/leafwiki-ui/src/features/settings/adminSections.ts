// Barrel for the admin-only settings sections (roles: ['admin']).
// Re-exported from one module so their `lazy()` imports in lazy-routes.tsx
// share a single dynamic-import specifier and land in one chunk instead of
// six — see the plan in pleaf/root/AI-Gen-Infos/plans/open/features/
// settings-admin-sections-shared-chunk.md. Account stays out of this barrel
// since it's visible to non-admin users too.
export { default as ApiKeysManagement } from '../apikeys/ApiKeysManagement'
export { default as BackupSettings } from '../backup/BackupSettings'
export { default as BrandingSettings } from '../branding/BrandingSettings'
export { default as Importer } from '../importer/Importer'
export { default as SnapshotSettings } from '../snapshot/SnapshotSettings'
export { default as UserManagement } from '../users/UserManagement'
