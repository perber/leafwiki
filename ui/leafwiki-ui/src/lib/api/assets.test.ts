import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockFetchWithAuth = vi.fn()
vi.mock('./auth', () => ({
  fetchWithAuth: (...args: unknown[]) => mockFetchWithAuth(...args),
}))

import {
  getPageAttachments,
  invalidatePageAttachments,
  listNonImageAttachments,
  uploadAsset,
} from './assets'

describe('listNonImageAttachments', () => {
  it('keeps non-image assets and normalizes URLs', () => {
    expect(
      listNonImageAttachments(['/assets/p1/notes.pdf', 'assets/p1/guide.docx']),
    ).toEqual([
      { name: 'notes.pdf', url: '/assets/p1/notes.pdf' },
      { name: 'guide.docx', url: '/assets/p1/guide.docx' },
    ])
  })

  it('filters out image extensions and deduplicates URLs', () => {
    expect(
      listNonImageAttachments([
        '/assets/p1/photo.png',
        '/assets/p1/notes.pdf',
        'assets/p1/notes.pdf',
      ]),
    ).toEqual([{ name: 'notes.pdf', url: '/assets/p1/notes.pdf' }])
  })
})

describe('getPageAttachments — caching', () => {
  beforeEach(() => {
    mockFetchWithAuth.mockReset()
    invalidatePageAttachments('p1')
  })

  it('fetches once and serves later reads from the cache', async () => {
    mockFetchWithAuth.mockResolvedValue({ files: ['/assets/p1/notes.pdf'] })

    const first = await getPageAttachments('p1')
    const second = await getPageAttachments('p1')

    expect(first).toEqual([{ name: 'notes.pdf', url: '/assets/p1/notes.pdf' }])
    expect(second).toBe(first)
    expect(mockFetchWithAuth).toHaveBeenCalledTimes(1)
  })

  it('deduplicates concurrent reads into a single request', async () => {
    mockFetchWithAuth.mockResolvedValue({ files: ['/assets/p1/notes.pdf'] })

    const [a, b] = await Promise.all([
      getPageAttachments('p1'),
      getPageAttachments('p1'),
    ])

    expect(a).toEqual(b)
    expect(mockFetchWithAuth).toHaveBeenCalledTimes(1)
  })

  it('refetches after a failed request instead of caching the failure', async () => {
    mockFetchWithAuth.mockRejectedValueOnce(new Error('boom'))
    await expect(getPageAttachments('p1')).rejects.toThrow('boom')

    mockFetchWithAuth.mockResolvedValue({ files: ['/assets/p1/notes.pdf'] })
    const files = await getPageAttachments('p1')

    expect(files).toEqual([{ name: 'notes.pdf', url: '/assets/p1/notes.pdf' }])
    expect(mockFetchWithAuth).toHaveBeenCalledTimes(2)
  })

  it('busts the cache when an asset is uploaded for the page', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({ files: [] })
    expect(await getPageAttachments('p1')).toEqual([])

    mockFetchWithAuth.mockResolvedValueOnce({ file: '/assets/p1/notes.pdf' })
    await uploadAsset('p1', new File(['x'], 'notes.pdf'))

    mockFetchWithAuth.mockResolvedValueOnce({ files: ['/assets/p1/notes.pdf'] })
    expect(await getPageAttachments('p1')).toEqual([
      { name: 'notes.pdf', url: '/assets/p1/notes.pdf' },
    ])
  })
})
