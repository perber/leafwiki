import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSnapshotStore } from './snapshot'

vi.mock('@/lib/api/snapshot', () => ({
  fetchSnapshotStatus: vi.fn(),
  fetchSnapshots: vi.fn(),
  triggerSnapshot: vi.fn(),
  deleteSnapshot: vi.fn(),
}))

import * as snapshotAPI from '@/lib/api/snapshot'

const initialState = useSnapshotStore.getState()

describe('useSnapshotStore', () => {
  beforeEach(() => {
    useSnapshotStore.setState(initialState, true)
    vi.clearAllMocks()
  })

  it('triggerNow polls status until the job finishes before reloading the list', async () => {
    ;(
      snapshotAPI.triggerSnapshot as ReturnType<typeof vi.fn>
    ).mockResolvedValue(undefined)
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: { isRunning: true, lastSnapshotAt: null, lastError: '' },
    })
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: {
        isRunning: false,
        lastSnapshotAt: '2026-08-09T12:00:00Z',
        lastError: '',
      },
    })
    ;(
      snapshotAPI.fetchSnapshots as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce([
      { id: 'snapshot-1', createdAt: '2026-08-09T12:00:00Z', sizeBytes: 100 },
    ])

    await useSnapshotStore.getState().triggerNow()

    expect(snapshotAPI.fetchSnapshotStatus).toHaveBeenCalledTimes(2)
    expect(snapshotAPI.fetchSnapshots).toHaveBeenCalledTimes(1)
    const state = useSnapshotStore.getState()
    expect(state.isRunning).toBe(false)
    expect(state.snapshots).toEqual([
      { id: 'snapshot-1', createdAt: '2026-08-09T12:00:00Z', sizeBytes: 100 },
    ])
  })

  it('does not poll further once the initial status already reports the job finished', async () => {
    ;(
      snapshotAPI.triggerSnapshot as ReturnType<typeof vi.fn>
    ).mockResolvedValue(undefined)
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: {
        isRunning: false,
        lastSnapshotAt: '2026-08-09T12:00:00Z',
        lastError: '',
      },
    })
    ;(
      snapshotAPI.fetchSnapshots as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce([])

    await useSnapshotStore.getState().triggerNow()

    expect(snapshotAPI.fetchSnapshotStatus).toHaveBeenCalledTimes(1)
    expect(snapshotAPI.fetchSnapshots).toHaveBeenCalledTimes(1)
  })

  it('still reloads the list even when the trigger POST itself fails (e.g. 409 already running)', async () => {
    ;(
      snapshotAPI.triggerSnapshot as ReturnType<typeof vi.fn>
    ).mockRejectedValue(new Error('already running'))
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: { isRunning: true, lastSnapshotAt: null, lastError: '' },
    })
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: {
        isRunning: false,
        lastSnapshotAt: '2026-08-09T12:00:00Z',
        lastError: '',
      },
    })
    ;(
      snapshotAPI.fetchSnapshots as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce([])

    await expect(useSnapshotStore.getState().triggerNow()).rejects.toThrow(
      'already running',
    )

    expect(snapshotAPI.fetchSnapshotStatus).toHaveBeenCalledTimes(2)
    expect(snapshotAPI.fetchSnapshots).toHaveBeenCalledTimes(1)
    expect(useSnapshotStore.getState().isRunning).toBe(false)
  })

  it('gives up polling after repeated errors but still reloads the list', async () => {
    ;(
      snapshotAPI.triggerSnapshot as ReturnType<typeof vi.fn>
    ).mockResolvedValue(undefined)
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      enabled: true,
      retentionCount: 10,
      status: { isRunning: true, lastSnapshotAt: null, lastError: '' },
    })
    ;(
      snapshotAPI.fetchSnapshotStatus as ReturnType<typeof vi.fn>
    ).mockRejectedValue(new Error('network error'))
    ;(
      snapshotAPI.fetchSnapshots as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce([])

    await useSnapshotStore.getState().triggerNow()

    // 1 initial (loadStatus) + 3 failing poll attempts (POLL_ERROR_LIMIT) before giving up.
    expect(snapshotAPI.fetchSnapshotStatus).toHaveBeenCalledTimes(4)
    expect(snapshotAPI.fetchSnapshots).toHaveBeenCalledTimes(1)
    // isRunning is left at whatever the last successful read reported (still true).
    expect(useSnapshotStore.getState().isRunning).toBe(true)
  }, 10000)
})
