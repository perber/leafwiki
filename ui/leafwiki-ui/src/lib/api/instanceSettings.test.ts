import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockFetchWithAuth = vi.fn()
vi.mock('./auth', () => ({
  fetchWithAuth: (...args: unknown[]) => mockFetchWithAuth(...args),
}))

import { updatePageDisplay } from './instanceSettings'

describe('updatePageDisplay', () => {
  beforeEach(() => {
    mockFetchWithAuth.mockReset()
  })

  it.each([
    { name: 'one field', patch: { contentWidth: 'wide' as const } },
    { name: 'the other field', patch: { alwaysShowToc: true } },
    {
      name: 'both fields',
      patch: { alwaysShowToc: false, contentWidth: 'default' as const },
    },
  ])(
    'PUTs $name to the admin display endpoint and returns the server reply',
    async ({ patch }) => {
      const reply = { alwaysShowToc: true, contentWidth: 'wide' }
      mockFetchWithAuth.mockResolvedValueOnce(reply)

      await expect(updatePageDisplay(patch)).resolves.toEqual(reply)
      expect(mockFetchWithAuth).toHaveBeenCalledWith(
        '/api/admin/settings/display',
        { method: 'PUT', body: JSON.stringify(patch) },
      )
    },
  )
})
