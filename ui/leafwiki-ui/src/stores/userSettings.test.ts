import type { UserSettings } from '@/lib/api/userSettings'
import i18next from '@/lib/i18n'
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from 'vitest'

vi.mock('@/lib/api/userSettings', () => ({
  getUserSettings: vi.fn(),
  updateUserSettings: vi.fn(),
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import * as userSettingsApi from '@/lib/api/userSettings'
import { toast } from 'sonner'
import { useUserSettingsStore } from './userSettings'

function makeSettings(overrides: Partial<UserSettings> = {}): UserSettings {
  return {
    userId: 'user-1',
    language: 'en',
    autoSave: true,
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('useUserSettingsStore', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    useUserSettingsStore.setState({
      autoSave: true,
      language: 'en',
      loaded: false,
    })
  })

  afterEach(async () => {
    await i18next.changeLanguage('en')
  })

  it('loadUserSettings applies the fetched language via i18next', async () => {
    ;(userSettingsApi.getUserSettings as Mock).mockResolvedValue(
      makeSettings({ language: 'de' }),
    )

    await useUserSettingsStore.getState().loadUserSettings()

    expect(useUserSettingsStore.getState().language).toBe('de')
    expect(useUserSettingsStore.getState().loaded).toBe(true)
    expect(i18next.language).toBe('de')
  })

  it('loadUserSettings ignores an unshipped language code', async () => {
    ;(userSettingsApi.getUserSettings as Mock).mockResolvedValue(
      makeSettings({ language: 'xx-not-a-real-language' }),
    )

    await useUserSettingsStore.getState().loadUserSettings()

    expect(i18next.language).toBe('en')
  })

  it('setLanguage optimistically switches the language before the API resolves', async () => {
    let resolveUpdate: (settings: UserSettings) => void = () => {}
    ;(userSettingsApi.updateUserSettings as Mock).mockReturnValue(
      new Promise((resolve) => {
        resolveUpdate = resolve
      }),
    )

    const promise = useUserSettingsStore.getState().setLanguage('de')
    expect(useUserSettingsStore.getState().language).toBe('de')
    expect(i18next.language).toBe('de')

    resolveUpdate(makeSettings({ language: 'de' }))
    await promise
    expect(userSettingsApi.updateUserSettings).toHaveBeenCalledWith({
      language: 'de',
    })
  })

  it('setLanguage rolls back the language and toasts on a failed update', async () => {
    ;(userSettingsApi.updateUserSettings as Mock).mockRejectedValue(
      new Error('boom'),
    )

    await useUserSettingsStore.getState().setLanguage('de')

    expect(useUserSettingsStore.getState().language).toBe('en')
    expect(i18next.language).toBe('en')
    expect(toast.error).toHaveBeenCalled()
  })

  it('clearUserSettings resets language back to the default', () => {
    useUserSettingsStore.setState({ language: 'de', loaded: true })

    useUserSettingsStore.getState().clearUserSettings()

    expect(useUserSettingsStore.getState().language).toBe('en')
    expect(useUserSettingsStore.getState().loaded).toBe(false)
  })
})
