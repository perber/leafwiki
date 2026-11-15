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
  updatePageDisplay: vi.fn(),
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

describe('pageDisplay', () => {
  it.each([
    { name: 'a known width', contentWidth: 'wide', want: 'wide' },
    {
      name: 'a missing field (older server)',
      contentWidth: undefined,
      want: 'default',
    },
    { name: 'an unknown width', contentWidth: 'huge', want: 'default' },
    // Same rule as the backend (exact match, no case folding or trimming).
    { name: 'a capitalised width', contentWidth: 'Wide', want: 'default' },
    { name: 'a padded width', contentWidth: ' wide', want: 'default' },
    { name: 'a null width', contentWidth: null, want: 'default' },
    { name: 'a non-string width', contentWidth: 1, want: 'default' },
    {
      name: 'an Object.prototype key',
      contentWidth: 'constructor',
      want: 'default',
    },
  ])('loadConfig keeps $name as $want', async ({ contentWidth, want }) => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce({
      ...baseConfig,
      contentWidth,
    })

    await useConfigStore.getState().loadConfig()

    expect(useConfigStore.getState().pageDisplay.contentWidth).toBe(want)
  })

  it.each([
    { name: 'true', alwaysShowToc: true, want: true },
    { name: 'false', alwaysShowToc: false, want: false },
    { name: 'a missing flag', alwaysShowToc: undefined, want: false },
  ])(
    'loadConfig reads alwaysShowToc $name as $want',
    async ({ alwaysShowToc, want }) => {
      ;(configApi.getConfig as Mock).mockResolvedValueOnce({
        ...baseConfig,
        alwaysShowToc,
      })

      await useConfigStore.getState().loadConfig()

      expect(useConfigStore.getState().pageDisplay.alwaysShowToc).toBe(want)
    },
  )

  it('updatePageDisplay sends the patch and takes only the patched fields from the reply', async () => {
    useConfigStore.setState({
      pageDisplay: { alwaysShowToc: false, contentWidth: 'default' },
    })
    ;(instanceSettingsApi.updatePageDisplay as Mock).mockResolvedValueOnce({
      alwaysShowToc: true,
      contentWidth: 'wide',
    })

    await useConfigStore.getState().updatePageDisplay({ contentWidth: 'wide' })

    expect(instanceSettingsApi.updatePageDisplay).toHaveBeenCalledWith({
      contentWidth: 'wide',
    })
    // alwaysShowToc was not in the patch: the reply's copy of it is ignored.
    expect(useConfigStore.getState().pageDisplay).toEqual({
      alwaysShowToc: false,
      contentWidth: 'wide',
    })
  })

  it('updatePageDisplay falls back to the default for an unknown confirmed width', async () => {
    useConfigStore.setState({
      pageDisplay: { alwaysShowToc: false, contentWidth: 'wide' },
    })
    ;(instanceSettingsApi.updatePageDisplay as Mock).mockResolvedValueOnce({
      alwaysShowToc: false,
      contentWidth: 'huge',
    })

    await useConfigStore.getState().updatePageDisplay({ contentWidth: 'wide' })

    expect(useConfigStore.getState().pageDisplay.contentWidth).toBe('default')
  })

  it('updatePageDisplay keeps the old settings when the request fails', async () => {
    const before = { alwaysShowToc: true, contentWidth: 'default' as const }
    useConfigStore.setState({ pageDisplay: before })
    ;(instanceSettingsApi.updatePageDisplay as Mock).mockRejectedValueOnce(
      new Error('boom'),
    )

    await expect(
      useConfigStore
        .getState()
        .updatePageDisplay({ alwaysShowToc: false, contentWidth: 'wide' }),
    ).rejects.toThrow('boom')
    expect(useConfigStore.getState().pageDisplay).toEqual(before)
  })

  it('updatePageDisplay leaves unrelated config fields alone', async () => {
    useConfigStore.setState({
      publicAccess: true,
      editorLimit: 7,
      defaultLanguage: 'de',
      pageDisplay: { alwaysShowToc: false, contentWidth: 'default' },
    })
    ;(instanceSettingsApi.updatePageDisplay as Mock).mockResolvedValueOnce({
      alwaysShowToc: true,
      contentWidth: 'default',
      // A reply carrying a non-display field must not leak into the store.
      publicAccess: false,
    })

    await useConfigStore.getState().updatePageDisplay({ alwaysShowToc: true })

    const state = useConfigStore.getState()
    expect(state.publicAccess).toBe(true)
    expect(state.editorLimit).toBe(7)
    expect(state.defaultLanguage).toBe('de')
  })

  // The TOC checkbox and the width picker have separate saving flags, so an
  // admin can have both requests in flight. Each reply carries all display
  // settings as of its own write; the store must end up with what the
  // server holds after both writes, whatever order the replies arrive in.
  it('two in-flight updates keep both changes when the replies arrive out of order', async () => {
    useConfigStore.setState({
      pageDisplay: { alwaysShowToc: false, contentWidth: 'default' },
    })
    let resolveToc: (v: unknown) => void = () => {}
    let resolveWidth: (v: unknown) => void = () => {}
    ;(instanceSettingsApi.updatePageDisplay as Mock)
      .mockImplementationOnce(() => new Promise((r) => (resolveToc = r)))
      .mockImplementationOnce(() => new Promise((r) => (resolveWidth = r)))

    const toc = useConfigStore
      .getState()
      .updatePageDisplay({ alwaysShowToc: true })
    const width = useConfigStore
      .getState()
      .updatePageDisplay({ contentWidth: 'wide' })
    // Server order: toc write first, then width write.
    resolveWidth({ alwaysShowToc: true, contentWidth: 'wide' })
    await width
    resolveToc({ alwaysShowToc: true, contentWidth: 'default' })
    await toc

    expect(useConfigStore.getState().pageDisplay).toEqual({
      alwaysShowToc: true,
      contentWidth: 'wide',
    })
  })
})
