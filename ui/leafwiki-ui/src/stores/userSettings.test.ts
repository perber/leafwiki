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
    dateFormat: 'locale',
    timeFormat: 'locale',
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

  it('loadUserSettings hydrates the date/time format preference', async () => {
    ;(userSettingsApi.getUserSettings as Mock).mockResolvedValue(
      makeSettings({ dateFormat: 'dmy_dot', timeFormat: '24h' }),
    )

    await useUserSettingsStore.getState().loadUserSettings()

    expect(useUserSettingsStore.getState().dateFormat).toBe('dmy_dot')
    expect(useUserSettingsStore.getState().timeFormat).toBe('24h')
  })

  it('setDateFormat optimistically updates and PUTs the patch', async () => {
    ;(userSettingsApi.updateUserSettings as Mock).mockResolvedValue(
      makeSettings({ dateFormat: 'iso' }),
    )

    await useUserSettingsStore.getState().setDateFormat('iso')

    expect(useUserSettingsStore.getState().dateFormat).toBe('iso')
    expect(userSettingsApi.updateUserSettings).toHaveBeenCalledWith({
      dateFormat: 'iso',
    })
  })

  it('setTimeFormat rolls back and toasts on a failed update', async () => {
    useUserSettingsStore.setState({ timeFormat: 'locale' })
    ;(userSettingsApi.updateUserSettings as Mock).mockRejectedValue(
      new Error('boom'),
    )

    await useUserSettingsStore.getState().setTimeFormat('12h')

    expect(useUserSettingsStore.getState().timeFormat).toBe('locale')
    expect(toast.error).toHaveBeenCalled()
  })

  it('a slow, failing earlier format write does not clobber a newer choice', async () => {
    useUserSettingsStore.setState({ dateFormat: 'locale' })
    let failFirst: (err: Error) => void = () => {}
    ;(userSettingsApi.updateUserSettings as Mock)
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            failFirst = reject
          }),
      )
      .mockResolvedValueOnce(makeSettings({ dateFormat: 'dmy_dot' }))

    const first = useUserSettingsStore.getState().setDateFormat('iso')
    const second = useUserSettingsStore.getState().setDateFormat('dmy_dot')
    expect(useUserSettingsStore.getState().dateFormat).toBe('dmy_dot')

    await vi.waitFor(() =>
      expect(userSettingsApi.updateUserSettings).toHaveBeenCalledTimes(1),
    )
    failFirst(new Error('boom'))
    await Promise.all([first, second])

    // The newer value wins, the stale failure neither rolls back nor toasts,
    // and its PUT ran before the newer one so the server ends up on 'dmy_dot'.
    expect(useUserSettingsStore.getState().dateFormat).toBe('dmy_dot')
    expect(toast.error).not.toHaveBeenCalled()
    expect(
      (userSettingsApi.updateUserSettings as Mock).mock.calls.map((c) => c[0]),
    ).toEqual([{ dateFormat: 'iso' }, { dateFormat: 'dmy_dot' }])
  })

  it('clearUserSettings resets the format preference to locale', () => {
    useUserSettingsStore.setState({ dateFormat: 'iso', timeFormat: '12h' })

    useUserSettingsStore.getState().clearUserSettings()

    expect(useUserSettingsStore.getState().dateFormat).toBe('locale')
    expect(useUserSettingsStore.getState().timeFormat).toBe('locale')
  })
})
