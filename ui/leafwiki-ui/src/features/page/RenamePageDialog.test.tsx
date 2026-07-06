import { DIALOG_RENAME_PAGE } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RenamePageDialog } from './RenamePageDialog'

vi.mock('@/lib/api/pages', () => ({
  NODE_KIND_PAGE: 'page',
  applyPageRefactor: vi.fn(),
  getPageByPath: vi.fn(),
  previewPageRefactor: vi.fn(),
  updatePage: vi.fn(),
}))

vi.mock('@/features/page/pageRefactorDialogState', () => ({
  confirmPageRefactor: vi.fn(),
}))

vi.mock('@/features/page/pageMutationRefresh', () => ({
  refreshAfterPageRefactor: vi.fn(),
}))

import {
  applyPageRefactor,
  getPageByPath,
  previewPageRefactor,
  updatePage,
} from '@/lib/api/pages'
import { confirmPageRefactor } from '@/features/page/pageRefactorDialogState'
import { refreshAfterPageRefactor } from '@/features/page/pageMutationRefresh'

const mockApplyPageRefactor = applyPageRefactor as ReturnType<typeof vi.fn>
const mockGetPageByPath = getPageByPath as ReturnType<typeof vi.fn>
const mockPreviewPageRefactor = previewPageRefactor as ReturnType<typeof vi.fn>
const mockUpdatePage = updatePage as ReturnType<typeof vi.fn>
const mockConfirmPageRefactor = confirmPageRefactor as ReturnType<typeof vi.fn>
const mockRefreshAfterPageRefactor = refreshAfterPageRefactor as ReturnType<
  typeof vi.fn
>

const samplePage = {
  id: 'page-1',
  kind: 'page' as const,
  title: 'Original title',
  slug: 'original-title',
  path: '/original-title',
  version: 'v1',
}

describe('RenamePageDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useConfigStore.setState({
      enableLinkRefactor: false,
    })
    useTreeStore.setState({
      reloadTree: vi.fn().mockResolvedValue(undefined),
    } as never)
    useDialogsStore.setState({
      dialogType: DIALOG_RENAME_PAGE,
      dialogProps: null,
    })
    mockGetPageByPath.mockResolvedValue({
      content: 'hello',
      tags: [],
      properties: {},
    })
    mockUpdatePage.mockResolvedValue({ ...samplePage })
    mockApplyPageRefactor.mockResolvedValue({ ...samplePage })
    mockPreviewPageRefactor.mockResolvedValue({
      kind: 'rename',
      pageId: 'page-1',
      oldPath: '/original-title',
      newPath: '/renamed-title',
      affectedPages: [],
      counts: {
        affectedPages: 0,
        matchedLinks: 0,
      },
      warnings: [],
    })
    mockConfirmPageRefactor.mockResolvedValue(false)
    mockRefreshAfterPageRefactor.mockResolvedValue(undefined)
  })

  it('updates the page title and slug directly when link refactor is disabled', async () => {
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/e/original-title']}>
        <RenamePageDialog
          page={{ ...samplePage, parentId: undefined, path: 'original-title' }}
        />
      </MemoryRouter>,
    )

    await user.clear(screen.getByTestId('rename-page-title-input'))
    await user.type(screen.getByTestId('rename-page-title-input'), 'Renamed title')
    await user.clear(screen.getByTestId('rename-page-slug-input'))
    await user.type(screen.getByTestId('rename-page-slug-input'), 'renamed-title')

    await user.click(
      screen.getByTestId('rename-page-dialog-button-confirm'),
    )

    expect(mockGetPageByPath).toHaveBeenCalledWith('original-title')
    expect(mockUpdatePage).toHaveBeenCalledWith(
      'page-1',
      'v1',
      'Renamed title',
      'renamed-title',
      'hello',
      [],
      {},
    )
    expect(mockApplyPageRefactor).not.toHaveBeenCalled()
    expect(mockRefreshAfterPageRefactor).toHaveBeenCalled()
  })

  it('uses page refactor flow when link refactor is enabled', async () => {
    useConfigStore.setState({
      enableLinkRefactor: true,
    })
    mockConfirmPageRefactor.mockResolvedValue(true)

    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/e/original-title']}>
        <RenamePageDialog
          page={{ ...samplePage, parentId: undefined, path: 'original-title' }}
        />
      </MemoryRouter>,
    )

    await user.clear(screen.getByTestId('rename-page-title-input'))
    await user.type(screen.getByTestId('rename-page-title-input'), 'Renamed title')
    await user.clear(screen.getByTestId('rename-page-slug-input'))
    await user.type(screen.getByTestId('rename-page-slug-input'), 'renamed-title')

    await user.click(
      screen.getByTestId('rename-page-dialog-button-confirm'),
    )

    expect(mockPreviewPageRefactor).toHaveBeenCalledWith('page-1', {
      kind: 'rename',
      title: 'Renamed title',
      slug: 'renamed-title',
    })
    expect(mockApplyPageRefactor).toHaveBeenCalledWith('page-1', {
      kind: 'rename',
      version: 'v1',
      title: 'Renamed title',
      slug: 'renamed-title',
      content: 'hello',
      rewriteLinks: true,
    })
    expect(mockUpdatePage).not.toHaveBeenCalled()
  })
})
