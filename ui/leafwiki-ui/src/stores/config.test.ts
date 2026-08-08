import * as configApi from '@/lib/api/config'
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

const baseConfig = {
  publicAccess: false,
  hideLinkMetadataSection: false,
  authDisabled: false,
  maxAssetUploadSizeBytes: 1000,
  enableRevision: false,
  enableLinkRefactor: false,
  enableApiKeyManagement: false,
  gitBackupEnabled: false,
  snapshotEnabled: false,
  totpAvailable: false,
  httpRemoteUserEnabled: true,
  loginUrl: '',
  logoutUrl: '',
  userManagementUrl: '',
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
  it('TestLoadConfig_TransientFailureThenSuccess_RetriesAndSucceeds', async () => {
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

  it('TestLoadConfig_PersistentFailure_GivesUpWithConfigLoadSucceededFalse', async () => {
    ;(configApi.getConfig as Mock).mockRejectedValue(new Error('down'))

    const loadPromise = useConfigStore.getState().loadConfig()
    await vi.runAllTimersAsync()
    await loadPromise

    expect(useConfigStore.getState().configLoadSucceeded).toBe(false)
    expect(useConfigStore.getState().hasLoaded).toBe(true)
    expect(useConfigStore.getState().httpRemoteUserEnabled).toBe(false)
    expect((configApi.getConfig as Mock).mock.calls.length).toBeGreaterThan(1)
  })

  it('TestLoadConfig_ImmediateSuccess_DoesNotRetry', async () => {
    ;(configApi.getConfig as Mock).mockResolvedValueOnce(baseConfig)

    await useConfigStore.getState().loadConfig()

    expect(useConfigStore.getState().configLoadSucceeded).toBe(true)
    expect(configApi.getConfig).toHaveBeenCalledTimes(1)
  })
})
