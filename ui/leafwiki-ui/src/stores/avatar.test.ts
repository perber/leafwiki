import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/api/avatar', () => ({
  uploadAvatar: vi.fn(),
  deleteAvatar: vi.fn(),
  avatarUrl: vi.fn(),
}))

import * as avatarAPI from '@/lib/api/avatar'
import { useAvatarStore } from './avatar'

function makeFile(name = 'avatar.png'): File {
  return new File(['fake-bytes'], name, { type: 'image/png' })
}

describe('useAvatarStore', () => {
  beforeEach(() => {
    useAvatarStore.setState({ avatarVersion: 0, isLoading: false, error: null })
    vi.mocked(avatarAPI.uploadAvatar).mockReset()
    vi.mocked(avatarAPI.deleteAvatar).mockReset()
  })

  it('uploadAvatar bumps avatarVersion on success', async () => {
    vi.mocked(avatarAPI.uploadAvatar).mockResolvedValue(undefined)

    await useAvatarStore.getState().uploadAvatar(makeFile())

    const state = useAvatarStore.getState()
    expect(state.avatarVersion).toBeGreaterThan(0)
    expect(state.isLoading).toBe(false)
    expect(state.error).toBeNull()
  })

  it('uploadAvatar sets an error and rethrows when the API call fails', async () => {
    vi.mocked(avatarAPI.uploadAvatar).mockRejectedValue(new Error('boom'))

    await expect(
      useAvatarStore.getState().uploadAvatar(makeFile()),
    ).rejects.toThrow('boom')

    const state = useAvatarStore.getState()
    expect(state.error).toBe('boom')
    expect(state.isLoading).toBe(false)
    // No prior client state to roll back to — version stays untouched.
    expect(state.avatarVersion).toBe(0)
  })

  it('deleteAvatar bumps avatarVersion on success', async () => {
    vi.mocked(avatarAPI.deleteAvatar).mockResolvedValue(undefined)

    await useAvatarStore.getState().deleteAvatar()

    const state = useAvatarStore.getState()
    expect(state.avatarVersion).toBeGreaterThan(0)
    expect(state.isLoading).toBe(false)
    expect(state.error).toBeNull()
  })

  it('deleteAvatar sets an error and rethrows when the API call fails', async () => {
    vi.mocked(avatarAPI.deleteAvatar).mockRejectedValue(new Error('boom'))

    await expect(useAvatarStore.getState().deleteAvatar()).rejects.toThrow(
      'boom',
    )

    const state = useAvatarStore.getState()
    expect(state.error).toBe('boom')
    expect(state.isLoading).toBe(false)
    expect(state.avatarVersion).toBe(0)
  })

  it('isLoading is true while the upload is in flight', async () => {
    let resolveUpload: () => void = () => {}
    vi.mocked(avatarAPI.uploadAvatar).mockReturnValue(
      new Promise((resolve) => {
        resolveUpload = () => resolve(undefined)
      }),
    )

    const promise = useAvatarStore.getState().uploadAvatar(makeFile())
    expect(useAvatarStore.getState().isLoading).toBe(true)

    resolveUpload()
    await promise
    expect(useAvatarStore.getState().isLoading).toBe(false)
  })
})
