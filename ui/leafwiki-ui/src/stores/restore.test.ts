import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useRestoreStore } from './restore'

vi.mock('@/lib/api/restore', () => ({
  triggerRestore: vi.fn(),
  triggerRestoreFromUpload: vi.fn(),
  getRestoreStatus: vi.fn(),
  triggerSelfRestart: vi.fn(),
}))
vi.mock('@/lib/api/resync', () => ({
  getResyncStatus: vi.fn(),
}))

import * as restoreAPI from '@/lib/api/restore'
import * as resyncAPI from '@/lib/api/resync'

const initialState = useRestoreStore.getState()

describe('useRestoreStore', () => {
  beforeEach(() => {
    useRestoreStore.setState(initialState, true)
    vi.clearAllMocks()
  })

  it('trigger(id) polls status until done, then the resync tail, and clears isLoading', async () => {
    ;(restoreAPI.triggerRestore as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    )
    ;(
      restoreAPI.getRestoreStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: 'swapping', done: false })
    ;(
      restoreAPI.getRestoreStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: null, done: true })
    ;(
      resyncAPI.getResyncStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: 'search', done: true })

    await useRestoreStore.getState().trigger('snapshot-1')

    expect(restoreAPI.triggerRestore).toHaveBeenCalledWith('snapshot-1')
    const state = useRestoreStore.getState()
    expect(state.isLoading).toBe(false)
    expect(state.needsIntervention).toBe(false)
    expect(state.resyncConfirmed).toBe(true)
  })

  it('triggerUpload(file) drives the same phase/resync-tail transitions as trigger(id)', async () => {
    const file = new File(['zip-bytes'], 'backup.zip')
    ;(
      restoreAPI.triggerRestoreFromUpload as ReturnType<typeof vi.fn>
    ).mockResolvedValue(undefined)
    ;(
      restoreAPI.getRestoreStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: 'validating', done: false })
    ;(
      restoreAPI.getRestoreStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: null, done: true })
    ;(
      resyncAPI.getResyncStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({ running: true, phase: 'search', done: true })

    await useRestoreStore.getState().triggerUpload(file)

    expect(restoreAPI.triggerRestoreFromUpload).toHaveBeenCalledWith(file)
    const state = useRestoreStore.getState()
    expect(state.isLoading).toBe(false)
    expect(state.needsIntervention).toBe(false)
    expect(state.resyncConfirmed).toBe(true)
  })

  it('triggerUpload(file) surfaces needsIntervention the same way trigger(id) does', async () => {
    const file = new File(['zip-bytes'], 'backup.zip')
    ;(
      restoreAPI.triggerRestoreFromUpload as ReturnType<typeof vi.fn>
    ).mockResolvedValue(undefined)
    ;(
      restoreAPI.getRestoreStatus as ReturnType<typeof vi.fn>
    ).mockResolvedValueOnce({
      running: false,
      phase: null,
      done: true,
      needsIntervention: true,
    })

    await useRestoreStore.getState().triggerUpload(file)

    const state = useRestoreStore.getState()
    expect(state.isLoading).toBe(false)
    expect(state.needsIntervention).toBe(true)
  })

  it('triggerUpload(file) rethrows and resets isLoading when the upload call itself fails', async () => {
    const file = new File(['zip-bytes'], 'backup.zip')
    const uploadError = new Error('upload failed')
    ;(
      restoreAPI.triggerRestoreFromUpload as ReturnType<typeof vi.fn>
    ).mockRejectedValue(uploadError)

    await expect(
      useRestoreStore.getState().triggerUpload(file),
    ).rejects.toThrow('upload failed')

    expect(useRestoreStore.getState().isLoading).toBe(false)
    expect(restoreAPI.getRestoreStatus).not.toHaveBeenCalled()
  })
})
