// settingsSectionRegistry
// The extension point for LeafWiki's unified /settings page (ADR-0007's
// first concrete instance). A new settings section is one lazy import plus
// one entry in `settingsSections` — no edits needed to the router, the user
// menu, or the app layout.

import { hasRole } from '@/lib/roles'
import {
  AccountSettings,
  ApiKeysManagement,
  BackupSettings,
  BrandingSettings,
  Importer,
  SnapshotSettings,
  UserManagement,
} from '@/features/router/lazy-routes'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import {
  Camera,
  GitBranch,
  KeyRound,
  Palette,
  Upload,
  User,
  Users,
  type LucideIcon,
} from 'lucide-react'
import type { ComponentType, LazyExoticComponent } from 'react'

export interface SettingsSectionContext {
  role: string | undefined
  authDisabled: boolean
  gitBackupEnabled: boolean
  snapshotEnabled: boolean
  enableApiKeyManagement: boolean
  totpAvailable: boolean
  userManagementUrl: string | undefined
  editorLimit: number
}

export interface SettingsSection {
  id: string
  // Path segment nested under /settings, e.g. 'branding' -> /settings/branding.
  path: string
  labelKey: string
  ns: string
  icon: LucideIcon
  // [] = any authenticated user, matching hasRole()'s convention.
  roles: string[]
  isEnabled?: (ctx: SettingsSectionContext) => boolean
  // Only set for sections that link out to an external URL instead of
  // rendering their own route (mirrors today's userManagementUrl case).
  externalHref?: (ctx: SettingsSectionContext) => string | undefined
  Component: LazyExoticComponent<ComponentType>
}

export function isSectionVisible(
  section: SettingsSection,
  ctx: SettingsSectionContext,
): boolean {
  // With auth disabled there are no accounts/roles at all (the "public
  // editor" deployment mode) — the old flat routes never gated on role in
  // that mode either, and it's the only way an auth-disabled instance can
  // reach e.g. branding settings, so role checks are skipped rather than
  // locking every section out.
  const roleOk = ctx.authDisabled || hasRole(ctx.role, section.roles)
  if (!roleOk) return false
  return section.isEnabled ? section.isEnabled(ctx) : true
}

export const settingsSections: SettingsSection[] = [
  {
    id: 'account',
    path: 'account',
    labelKey: 'nav.account',
    ns: 'settings',
    icon: User,
    roles: [],
    Component: AccountSettings,
  },
  {
    id: 'branding',
    path: 'branding',
    labelKey: 'userMenu.brandingSettings',
    ns: 'auth',
    icon: Palette,
    roles: ['admin'],
    Component: BrandingSettings,
  },
  {
    id: 'users',
    path: 'users',
    labelKey: 'userMenu.userManagement',
    ns: 'auth',
    icon: Users,
    roles: ['admin'],
    // At editorLimit === 1, this plan allows only the one owner/admin — no
    // one can ever be added, so the section (and the loophole of adding
    // users this way instead of through the enforced /api/users limit) is
    // hidden entirely rather than shown empty. See ErrEditorLimitReached
    // (internal/core/auth) for the backend enforcement this mirrors.
    isEnabled: (ctx) => ctx.editorLimit !== 1,
    externalHref: (ctx) => ctx.userManagementUrl,
    Component: UserManagement,
  },
  {
    id: 'api-keys',
    path: 'api-keys',
    labelKey: 'menu.title',
    ns: 'apikeys',
    icon: KeyRound,
    roles: ['admin'],
    isEnabled: (ctx) => ctx.enableApiKeyManagement,
    Component: ApiKeysManagement,
  },
  {
    id: 'backup',
    path: 'backup',
    labelKey: 'menuLabel',
    ns: 'backup',
    icon: GitBranch,
    roles: ['admin'],
    isEnabled: (ctx) => ctx.gitBackupEnabled,
    Component: BackupSettings,
  },
  {
    id: 'snapshots',
    path: 'snapshots',
    labelKey: 'menuLabel',
    ns: 'snapshot',
    icon: Camera,
    roles: ['admin'],
    isEnabled: (ctx) => ctx.snapshotEnabled,
    Component: SnapshotSettings,
  },
  {
    id: 'importer',
    path: 'importer',
    labelKey: 'userMenu.import',
    ns: 'auth',
    icon: Upload,
    roles: ['admin'],
    Component: Importer,
  },
]

export function useSettingsSectionContext(): SettingsSectionContext {
  const role = useSessionStore((s) => s.user?.role)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const gitBackupEnabled = useConfigStore((s) => s.gitBackupEnabled)
  const snapshotEnabled = useConfigStore((s) => s.snapshotEnabled)
  const enableApiKeyManagement = useConfigStore((s) => s.enableApiKeyManagement)
  const totpAvailable = useConfigStore((s) => s.totpAvailable)
  const userManagementUrl = useConfigStore((s) => s.userManagementUrl)
  const editorLimit = useConfigStore((s) => s.editorLimit)

  return {
    role,
    authDisabled,
    gitBackupEnabled,
    snapshotEnabled,
    enableApiKeyManagement,
    totpAvailable,
    userManagementUrl: userManagementUrl || undefined,
    editorLimit,
  }
}
