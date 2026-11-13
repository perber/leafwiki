import * as configApi from '@/lib/api/config'
import * as instanceSettingsApi from '@/lib/api/instanceSettings'
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
import { useConfigStore } from './config'

vi.mock('@/lib/api/config', () => ({
  getConfig: vi.fn(),
}))

vi.mock('@/lib/api/instanceSettings', () => ({
  setContentWidth: vi.fn(),
}))

const baseConfig = {
  publicAccess: false,
  publicAccessEnvManaged: false,
  hideLinkMetadataSection: false,
  authDisabled: false,
  maxAssetUploadSizeBytes: 1000,
  enableRevision: false,
  enableLinkRefactor: false,
  enableApiKeyManagement: false,
  gitBackupEnabled: false,
  snapshotEnabled: false,
  smtpEnabled: false,
  totpAvailable: false,
  httpRemoteUserEnabled: true,
  loginUrl: '',
  logoutUrl: '',
  userManagementUrl: '',
  defaultLanguage: '',
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
  useConfigStore.setState({
    hasLoaded: false,
    configLoadSucceeded: false,
    httpRemoteUserEnabled: false,
    error: null,
  })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('loadConfig retry', () => {
  it('retries and succeeds after a transient /api/config failure', async () => {
    ;(configApi.getConfig as Mock)
      .mockRejectedValueOnce(new Error('network blip'))
      .mockResolvedValueOnce(baseConfig)

    const loadPromise = useConfigStore.getState().loadConfig()
    await vi.runAllTimersAsync()
    await loadPromise

    expect(useConfigStore.getState().configLoadSucceeded).toBe(true)
    expect(useConfigStore.getState().hasLoaded).toBe(true)
    expect(useConfigStore.getState().httpRemoteUserEnabled).toBe(true)
    expect(configApi.getConfig).toHaveBeenCalledTimes(2)
  })

  it('gives up with configLoadSucceeded false after repeated failures', async () => {
    ;(configApi.getConfig as Mock).mockRejectedValue(new Error('down'))

    const loadPromise = useConfigStore.getState().loadConfig()
    await vi.runAllTimersAsync()
    await loadPromise

    expect(useConfigStore.getState().configLoadSucceeded).toBe(false)
    expect(useConfigStore.getState().hasLoaded).toBe(true)
    expect(useConfigStore.getState().httpRemoteUserEnabled).toBe(false)
    expect((configApi.getConfig as Mock).mock.calls.length).toBeGreaterThan(1)
  })

  it('does not retry when the first attempt succeeds', async () => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce(baseConfig)

    await useConfigStore.getState().loadConfig()

    expect(useConfigStore.getState().configLoadSucceeded).toBe(true)
    expect(configApi.getConfig).toHaveBeenCalledTimes(1)
  })
})

describe('loadConfig defaultLanguage', () => {
  afterEach(async () => {
    await i18next.changeLanguage('en')
  })

  it('switches the UI language when defaultLanguage is a shipped language', async () => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce({
      ...baseConfig,
      defaultLanguage: 'de',
    })

    await useConfigStore.getState().loadConfig()

    expect(i18next.language).toBe('de')
    expect(useConfigStore.getState().defaultLanguage).toBe('de')
  })

  it('leaves the UI language unchanged when defaultLanguage is empty', async () => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce(baseConfig)

    await useConfigStore.getState().loadConfig()

    expect(i18next.language).toBe('en')
  })

  it('ignores an unrecognized defaultLanguage', async () => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce({
      ...baseConfig,
      defaultLanguage: 'xx',
    })

    await useConfigStore.getState().loadConfig()

    expect(i18next.language).toBe('en')
    expect(useConfigStore.getState().defaultLanguage).toBe('xx')
  })
})

describe('contentWidth', () => {
  it.each([
    { name: 'a known width', contentWidth: 'wide', want: 'wide' },
    {
      name: 'a missing field (older server)',
      contentWidth: undefined,
      want: 'default',
    },
    { name: 'an unknown width', contentWidth: 'huge', want: 'default' },
  ])('loadConfig keeps $name as $want', async ({ contentWidth, want }) => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce({
      ...baseConfig,
      contentWidth,
    })

    await useConfigStore.getState().loadConfig()

    expect(useConfigStore.getState().contentWidth).toBe(want)
  })

  it('setContentWidth stores the width the server confirmed', async () => {
    useConfigStore.setState({ contentWidth: 'default' })
    ;(instanceSettingsApi.setContentWidth as Mock).mockResolvedValueOnce({
      contentWidth: 'wide',
    })

    await useConfigStore.getState().setContentWidth('wide')

    expect(instanceSettingsApi.setContentWidth).toHaveBeenCalledWith('wide')
    expect(useConfigStore.getState().contentWidth).toBe('wide')
  })

  it('setContentWidth keeps the old width when the request fails', async () => {
    useConfigStore.setState({ contentWidth: 'default' })
    ;(instanceSettingsApi.setContentWidth as Mock).mockRejectedValueOnce(
      new Error('boom'),
    )

    await expect(
      useConfigStore.getState().setContentWidth('wide'),
    ).rejects.toThrow('boom')
    expect(useConfigStore.getState().contentWidth).toBe('default')
  })
})
