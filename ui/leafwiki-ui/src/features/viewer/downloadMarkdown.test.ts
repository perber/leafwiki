import { describe, expect, it, vi } from 'vitest'
import type { Page } from '@/lib/api/pages'
import {
  downloadPageMarkdown,
  getMarkdownDownloadFilename,
} from './downloadMarkdown'

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

describe('getMarkdownDownloadFilename', () => {
  it('uses the last path segment as a markdown filename', () => {
    expect(
      getMarkdownDownloadFilename(
        page({ path: 'docs/getting-started', slug: 'ignored' }),
      ),
    ).toBe('getting-started.md')
  })

  it('sanitizes unsafe filename characters', () => {
    expect(
      getMarkdownDownloadFilename(
        page({ path: 'docs/API: Keys?', slug: 'ignored' }),
      ),
    ).toBe('api-keys.md')
  })

  it('falls back to the title when path and slug are blank', () => {
    expect(
      getMarkdownDownloadFilename(
        page({ path: '', slug: '', title: 'Welcome Page' }),
      ),
    ).toBe('welcome-page.md')
  })
})

describe('downloadPageMarkdown', () => {
  it('downloads the page content as markdown', async () => {
    const objectUrl = 'blob:leafwiki-page'
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue(objectUrl)
    const revokeObjectURL = vi
      .spyOn(URL, 'revokeObjectURL')
      .mockImplementation(() => {})
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => {})

    downloadPageMarkdown(
      page({
        path: 'docs/example',
        content: '# Example\n\nMarkdown body\n',
      }),
    )

    expect(createObjectURL).toHaveBeenCalledTimes(1)
    const blob = createObjectURL.mock.calls[0][0] as Blob
    await expect(blob.text()).resolves.toBe('# Example\n\nMarkdown body\n')

    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith(objectUrl)

    createObjectURL.mockRestore()
    revokeObjectURL.mockRestore()
    click.mockRestore()
  })
})
