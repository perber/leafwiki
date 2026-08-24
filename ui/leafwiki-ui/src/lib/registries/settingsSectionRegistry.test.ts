import { describe, expect, it } from 'vitest'
import {
  isSectionVisible,
  settingsSections,
  type SettingsSection,
  type SettingsSectionContext,
} from './settingsSectionRegistry'

const baseCtx: SettingsSectionContext = {
  role: 'admin',
  authDisabled: false,
  gitBackupEnabled: false,
  snapshotEnabled: false,
  enableApiKeyManagement: false,
  totpAvailable: false,
  userManagementUrl: undefined,
  editorLimit: 0,
}

function section(overrides: Partial<SettingsSection> = {}): SettingsSection {
  return {
    id: 'test',
    path: 'test',
    labelKey: 'test',
    ns: 'settings',
    icon: (settingsSections[0] as SettingsSection).icon,
    roles: [],
    Component: settingsSections[0].Component,
    ...overrides,
  }
}

describe('isSectionVisible', () => {
  it('is visible for any authenticated user when roles is empty', () => {
    expect(
      isSectionVisible(section({ roles: [] }), { ...baseCtx, role: 'viewer' }),
    ).toBe(true)
  })

  it('is hidden for a role not in the allowed list', () => {
    expect(
      isSectionVisible(section({ roles: ['admin'] }), {
        ...baseCtx,
        role: 'editor',
      }),
    ).toBe(false)
  })

  it('is visible for a role in the allowed list', () => {
    expect(
      isSectionVisible(section({ roles: ['admin'] }), {
        ...baseCtx,
        role: 'admin',
      }),
    ).toBe(true)
  })

  it('is hidden when isEnabled returns false, even for an allowed role', () => {
    expect(
      isSectionVisible(
        section({ roles: ['admin'], isEnabled: (ctx) => ctx.gitBackupEnabled }),
        { ...baseCtx, role: 'admin', gitBackupEnabled: false },
      ),
    ).toBe(false)
  })

  it('is visible when isEnabled returns true and the role matches', () => {
    expect(
      isSectionVisible(
        section({ roles: ['admin'], isEnabled: (ctx) => ctx.gitBackupEnabled }),
        { ...baseCtx, role: 'admin', gitBackupEnabled: true },
      ),
    ).toBe(true)
  })

  it('bypasses role gating entirely when auth is disabled (no accounts exist in that mode)', () => {
    expect(
      isSectionVisible(section({ roles: ['admin'] }), {
        ...baseCtx,
        authDisabled: true,
        role: undefined,
      }),
    ).toBe(true)
  })

  it('still enforces isEnabled feature flags even when auth is disabled', () => {
    expect(
      isSectionVisible(
        section({ roles: ['admin'], isEnabled: (ctx) => ctx.gitBackupEnabled }),
        {
          ...baseCtx,
          authDisabled: true,
          role: undefined,
          gitBackupEnabled: false,
        },
      ),
    ).toBe(false)
  })

  it('has no built-in bearing on externalHref — visibility and link-vs-route are independent', () => {
    const usersSection = settingsSections.find((s) => s.id === 'users')!
    expect(
      isSectionVisible(usersSection, {
        ...baseCtx,
        role: 'admin',
        userManagementUrl: 'https://control-plane.example.com/users',
      }),
    ).toBe(true)
    expect(
      usersSection.externalHref?.({
        ...baseCtx,
        userManagementUrl: 'https://control-plane.example.com/users',
      }),
    ).toBe('https://control-plane.example.com/users')
  })
})

describe('settingsSections gating (regression for the pre-registry backup/snapshots URL-bypass gap)', () => {
  it('gates backup behind gitBackupEnabled', () => {
    const backup = settingsSections.find((s) => s.id === 'backup')!
    expect(
      isSectionVisible(backup, { ...baseCtx, gitBackupEnabled: false }),
    ).toBe(false)
    expect(
      isSectionVisible(backup, { ...baseCtx, gitBackupEnabled: true }),
    ).toBe(true)
  })

  it('gates snapshots behind snapshotEnabled', () => {
    const snapshots = settingsSections.find((s) => s.id === 'snapshots')!
    expect(
      isSectionVisible(snapshots, { ...baseCtx, snapshotEnabled: false }),
    ).toBe(false)
    expect(
      isSectionVisible(snapshots, { ...baseCtx, snapshotEnabled: true }),
    ).toBe(true)
  })

  it('gates api-keys behind enableApiKeyManagement', () => {
    const apiKeys = settingsSections.find((s) => s.id === 'api-keys')!
    expect(
      isSectionVisible(apiKeys, { ...baseCtx, enableApiKeyManagement: false }),
    ).toBe(false)
    expect(
      isSectionVisible(apiKeys, { ...baseCtx, enableApiKeyManagement: true }),
    ).toBe(true)
  })

  it('hides users when editorLimit is 1 (a Solo plan can never have a second editor)', () => {
    const users = settingsSections.find((s) => s.id === 'users')!
    expect(isSectionVisible(users, { ...baseCtx, editorLimit: 1 })).toBe(false)
  })

  it('shows users when editorLimit is 0 (unlimited/self-hosted) or above 1 (e.g. a Team plan)', () => {
    const users = settingsSections.find((s) => s.id === 'users')!
    expect(isSectionVisible(users, { ...baseCtx, editorLimit: 0 })).toBe(true)
    expect(isSectionVisible(users, { ...baseCtx, editorLimit: 10 })).toBe(true)
  })

  it('makes account visible to any authenticated role, unlike the admin-only sections', () => {
    const account = settingsSections.find((s) => s.id === 'account')!
    expect(isSectionVisible(account, { ...baseCtx, role: 'viewer' })).toBe(true)
    expect(isSectionVisible(account, { ...baseCtx, role: 'editor' })).toBe(true)
    expect(isSectionVisible(account, { ...baseCtx, role: 'admin' })).toBe(true)
  })

  it('keeps admin-only sections hidden from non-admins', () => {
    const adminOnlyIds = [
      'branding',
      'users',
      'api-keys',
      'backup',
      'snapshots',
      'importer',
    ]
    for (const id of adminOnlyIds) {
      const s = settingsSections.find((sec) => sec.id === id)!
      expect(
        isSectionVisible(s, {
          ...baseCtx,
          role: 'editor',
          gitBackupEnabled: true,
          snapshotEnabled: true,
          enableApiKeyManagement: true,
        }),
      ).toBe(false)
    }
  })
})
