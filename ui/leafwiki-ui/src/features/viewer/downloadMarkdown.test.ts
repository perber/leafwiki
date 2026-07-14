import { beforeEach, describe, expect, it, vi } from 'vitest'
import { downloadPageFile, type Page } from '@/lib/api/pages'
import {
  downloadPageMarkdown,
  getDownloadFallbackFilename,
} from './downloadMarkdown'

vi.mock('@/lib/api/pages', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/pages')>()
  return { ...actual, downloadPageFile: vi.fn() }
})

const page = (overrides: Partial<Page>): Page => ({
  id: 'page-1',
  slug: 'page',
  path: 'docs/page',
  title: 'Page',
  content: '# Page\n',
  version: '1',
  kind: 'page',
  ...overrides,
})

describe('getDownloadFallbackFilename', () => {
  it('uses the last path segment with a .md extension for pages', () => {
    expect(
      getDownloadFallbackFilename(
        page({ path: 'docs/getting-started', slug: 'ignored' }),
      ),
    ).toBe('getting-started.md')
  })

  it('uses a .zip extension for sections', () => {
    expect(
      getDownloadFallbackFilename(
        page({ path: 'docs/guides', slug: 'ignored', kind: 'section' }),
      ),
    ).toBe('guides.zip')
  })

  it('sanitizes unsafe filename characters', () => {
    expect(
      getDownloadFallbackFilename(
        page({ path: 'docs/API: Keys?', slug: 'ignored' }),
      ),
    ).toBe('api-keys.md')
  })

  it('falls back to the title when path and slug are blank', () => {
    expect(
      getDownloadFallbackFilename(
        page({ path: '', slug: '', title: 'Welcome Page' }),
      ),
    ).toBe('welcome-page.md')
  })
})

describe('downloadPageMarkdown', () => {
  beforeEach(() => {
    vi.mocked(downloadPageFile).mockReset()
  })

  it('downloads the file returned by the API using the server filename', async () => {
    const blob = new Blob(['# Example\n\nMarkdown body\n'], {
      type: 'text/markdown',
    })
    vi.mocked(downloadPageFile).mockResolvedValue({
      blob,
      filename: 'example.md',
    })

    const objectUrl = 'blob:leafwiki-page'
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue(objectUrl)
    const revokeObjectURL = vi
      .spyOn(URL, 'revokeObjectURL')
      .mockImplementation(() => {})
    let capturedName: string | undefined
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(function (this: HTMLAnchorElement) {
        capturedName = this.download
      })

    await downloadPageMarkdown(page({ id: 'p1', path: 'docs/example' }))

    expect(downloadPageFile).toHaveBeenCalledWith('p1')
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(capturedName).toBe('example.md')
    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith(objectUrl)

    createObjectURL.mockRestore()
    revokeObjectURL.mockRestore()
    click.mockRestore()
  })

  it('falls back to a derived filename when the server omits one', async () => {
    vi.mocked(downloadPageFile).mockResolvedValue({
      blob: new Blob(['content']),
      filename: null,
    })

    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:leafwiki-section')
    const revokeObjectURL = vi
      .spyOn(URL, 'revokeObjectURL')
      .mockImplementation(() => {})
    let capturedName: string | undefined
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(function (this: HTMLAnchorElement) {
        capturedName = this.download
      })

    await downloadPageMarkdown(
      page({ id: 's1', path: 'docs/guides', kind: 'section' }),
    )

    expect(downloadPageFile).toHaveBeenCalledWith('s1')
    expect(capturedName).toBe('guides.zip')

    createObjectURL.mockRestore()
    revokeObjectURL.mockRestore()
    click.mockRestore()
  })
})
