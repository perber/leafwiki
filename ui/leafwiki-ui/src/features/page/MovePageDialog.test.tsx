import { DIALOG_MOVE_PAGE } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PageNode, PageRefactorPreview } from '@/lib/api/pages'
import {
  applyPageRefactor,
  movePage,
  previewPageRefactor,
} from '@/lib/api/pages'
import { MovePageDialog } from './MovePageDialog'
import { confirmPageRefactor } from './pageRefactorDialogState'
import { refreshAfterPageRefactor } from './pageMutationRefresh'

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    movePage: vi.fn(),
    previewPageRefactor: vi.fn(),
    applyPageRefactor: vi.fn(),
  }
})

vi.mock('./pageRefactorDialogState', () => ({
  confirmPageRefactor: vi.fn(),
}))

vi.mock('./pageMutationRefresh', () => ({
  refreshAfterPageRefactor: vi.fn().mockResolvedValue(undefined),
}))

Element.prototype.scrollIntoView = () => {}

const child: PageNode = {
  id: 'page-1',
  title: 'Child',
  slug: 'child',
  path: 'section-a/child',
  version: 'v1',
  parentId: 'section-a',
  children: null,
  kind: 'page',
}
const sectionA: PageNode = {
  id: 'section-a',
  title: 'Section A',
  slug: 'section-a',
  path: 'section-a',
  version: 'va',
  parentId: null,
  children: [child],
  kind: 'section',
}
const sectionB: PageNode = {
  id: 'section-b',
  title: 'Section B',
  slug: 'section-b',
  path: 'section-b',
  version: 'vb',
  parentId: null,
  children: [],
  kind: 'section',
}
const root: PageNode = {
  id: 'root',
  title: 'Root',
  slug: '',
  path: '',
  version: 'vr',
  parentId: null,
  children: [sectionA, sectionB],
  kind: 'section',
}

const preview: PageRefactorPreview = {
  kind: 'move',
  pageId: 'page-1',
  oldPath: 'section-a/child',
  newPath: 'section-b/child',
  affectedPages: [
    {
      fromPageId: 'linker-1',
      fromTitle: 'Linker',
      fromPath: 'linker',
      matchedPaths: ['/section-a/child'],
      warnings: [],
    },
  ],
  counts: { affectedPages: 1, matchedLinks: 1 },
  warnings: [],
}

describe('MovePageDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDialogsStore.setState({
      dialogType: DIALOG_MOVE_PAGE,
      dialogProps: null,
    })
    useTreeStore.setState({
      tree: root,
      byId: {
        'page-1': child,
        'section-a': sectionA,
        'section-b': sectionB,
        root,
      },
    })
  })

  it('allows skipping the link rewrite when confirming a move', async () => {
    useConfigStore.setState({ enableLinkRefactor: true })
    vi.mocked(previewPageRefactor).mockResolvedValue(preview)
    vi.mocked(confirmPageRefactor).mockResolvedValue(false)
    vi.mocked(applyPageRefactor).mockResolvedValue(null)

    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <MovePageDialog pageId="page-1" />
      </MemoryRouter>,
    )

    await user.click(screen.getByText('Section B'))
    await user.click(screen.getByTestId('move-page-dialog-button-confirm'))

    expect(confirmPageRefactor).toHaveBeenCalledWith(preview, {
      allowSkipRewrite: true,
    })
    expect(applyPageRefactor).toHaveBeenCalledWith(
      'page-1',
      expect.objectContaining({ rewriteLinks: false }),
    )
    expect(refreshAfterPageRefactor).toHaveBeenCalled()
  })

  it('falls back to a plain move without the refactor pipeline when link refactor is disabled', async () => {
    useConfigStore.setState({ enableLinkRefactor: false })
    vi.mocked(movePage).mockResolvedValue(null)

    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <MovePageDialog pageId="page-1" />
      </MemoryRouter>,
    )

    await user.click(screen.getByText('Section B'))
    await user.click(screen.getByTestId('move-page-dialog-button-confirm'))

    expect(movePage).toHaveBeenCalledWith('page-1', 'v1', 'section-b')
    expect(previewPageRefactor).not.toHaveBeenCalled()
    expect(confirmPageRefactor).not.toHaveBeenCalled()
  })
})
