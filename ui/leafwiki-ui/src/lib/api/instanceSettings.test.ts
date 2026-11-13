import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockFetchWithAuth = vi.fn()
vi.mock('./auth', () => ({
  fetchWithAuth: (...args: unknown[]) => mockFetchWithAuth(...args),
}))

import { setContentWidth } from './instanceSettings'

describe('setContentWidth', () => {
  beforeEach(() => {
    mockFetchWithAuth.mockReset()
  })

  it('PUTs the width to the admin content-width endpoint and returns the stored value', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({ contentWidth: 'wide' })

    await expect(setContentWidth('wide')).resolves.toEqual({
      contentWidth: 'wide',
    })
    expect(mockFetchWithAuth).toHaveBeenCalledWith(
      '/api/admin/settings/content-width',
      { method: 'PUT', body: JSON.stringify({ contentWidth: 'wide' }) },
    )
  })
})
