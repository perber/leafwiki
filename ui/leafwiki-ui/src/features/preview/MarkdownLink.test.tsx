import type { PageNode } from '@/lib/api/pages'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { act, render, screen } from '@testing-library/react'
import { fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MarkdownLink } from './MarkdownLink'

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    suggestSlug: vi.fn(),
  }
})

vi.mock('react-i18next', () => ({
  initReactI18next: {
    type: '3rdParty',
    init: () => {},
  },
  useTranslation: () => ({ t: (key: string) => key }),
}))

import { suggestSlug } from '@/lib/api/pages'
const mockSuggestSlug = suggestSlug as ReturnType<typeof vi.fn>

function makeFolder(id: string, path: string): PageNode {
  return {
    id,
    title: path,
    slug: path,
    path,
    version: 'v1',
    children: null,
    kind: 'section',
    parentId: null,
  }
}

function makePage(id: string, path: string, parentId: string | null): PageNode {
  return {
    id,
    title: path,
    slug: path,
    path,
    version: 'v1',
    children: null,
    kind: 'page',
    parentId,
  }
}

describe('MarkdownLink wikilink-notfound', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ byId: {}, byPath: {} })
  })

  it('creates the new page in the current page folder, not the root', async () => {
    const folder = makeFolder('folder-1', 'docs')
    const currentPage = makePage('page-1', 'docs/current', 'folder-1')
    useTreeStore.setState({
      byId: { 'folder-1': folder, 'page-1': currentPage },
      byPath: { docs: folder, 'docs/current': currentPage },
    })
    mockSuggestSlug.mockResolvedValue('tcp-ip')

    render(
      <MemoryRouter>
        <MarkdownLink href="wikilink-notfound:TCP%2FIP" path="docs/current">
          TCP/IP
        </MarkdownLink>
      </MemoryRouter>,
    )

    await act(async () => {
      fireEvent.click(screen.getByRole('button'))
      await Promise.resolve()
    })

    expect(mockSuggestSlug).toHaveBeenCalledWith('folder-1', 'TCP/IP')
    expect(useDialogsStore.getState().dialogType).toBe(
      DIALOG_CREATE_PAGE_BY_PATH,
    )
    expect(useDialogsStore.getState().dialogProps).toMatchObject({
      initialPath: 'docs/tcp-ip',
      initialTitle: 'TCP/IP',
    })
  })

  it('creates the new page at the root when the current page is top-level', async () => {
    const currentPage = makePage('page-2', 'root-page', null)
    useTreeStore.setState({
      byId: { 'page-2': currentPage },
      byPath: { 'root-page': currentPage },
    })
    mockSuggestSlug.mockResolvedValue('foo')

    render(
      <MemoryRouter>
        <MarkdownLink href="wikilink-notfound:Foo" path="root-page">
          Foo
        </MarkdownLink>
      </MemoryRouter>,
    )

    await act(async () => {
      fireEvent.click(screen.getByRole('button'))
      await Promise.resolve()
    })

    expect(mockSuggestSlug).toHaveBeenCalledWith('', 'Foo')
    expect(useDialogsStore.getState().dialogProps).toMatchObject({
      initialPath: 'foo',
      initialTitle: 'Foo',
    })
  })
})
