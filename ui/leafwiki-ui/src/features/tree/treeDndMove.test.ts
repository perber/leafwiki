import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  applyPageRefactor,
  movePage,
  previewPageRefactor,
} from '@/lib/api/pages'
import type { PageRefactorPreview } from '@/lib/api/pages'
import { confirmPageRefactor } from '@/features/page/pageRefactorDialogState'
import { useConfigStore } from '@/stores/config'
import { performTreeCrossParentMove } from './treeDndMove'
import { ROOT_ID } from './treeDndUtils'

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

vi.mock('@/features/page/pageRefactorDialogState', () => ({
  confirmPageRefactor: vi.fn(),
}))

const dragged = { id: 'page-1', version: 'v1' }
const preview: PageRefactorPreview = {
  kind: 'move',
  pageId: 'page-1',
  oldPath: 'section-a/page-1',
  newPath: 'section-b/page-1',
  affectedPages: [
    {
      fromPageId: 'linker-1',
      fromTitle: 'Linker',
      fromPath: 'linker',
      matchedPaths: ['/section-a/page-1'],
      warnings: [],
    },
  ],
  counts: { affectedPages: 1, matchedLinks: 1 },
  warnings: [],
}
const noAffectedPreview: PageRefactorPreview = {
  ...preview,
  affectedPages: [],
  counts: { affectedPages: 0, matchedLinks: 0 },
}

describe('performTreeCrossParentMove', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls plain movePage when link refactor is disabled', async () => {
    useConfigStore.setState({ enableLinkRefactor: false })
    vi.mocked(movePage).mockResolvedValue(null)

    const result = await performTreeCrossParentMove(dragged, {
      parentId: 'section-b',
      index: 2,
    })

    expect(movePage).toHaveBeenCalledWith('page-1', 'v1', 'section-b', 2)
    expect(previewPageRefactor).not.toHaveBeenCalled()
    expect(result).toEqual({ status: 'moved', preview: null })
  })

  it('applies with rewriteLinks:false when nothing is affected (no dialog shown)', async () => {
    useConfigStore.setState({ enableLinkRefactor: true })
    vi.mocked(previewPageRefactor).mockResolvedValue(noAffectedPreview)
    vi.mocked(confirmPageRefactor).mockResolvedValue(false)
    vi.mocked(applyPageRefactor).mockResolvedValue(null)

    const result = await performTreeCrossParentMove(dragged, {
      parentId: 'section-b',
      index: 2,
    })

    expect(previewPageRefactor).toHaveBeenCalledWith('page-1', {
      kind: 'move',
      parentId: 'section-b',
    })
    expect(confirmPageRefactor).toHaveBeenCalledWith(noAffectedPreview, {
      allowSkipRewrite: true,
    })
    expect(applyPageRefactor).toHaveBeenCalledWith('page-1', {
      kind: 'move',
      version: 'v1',
      parentId: 'section-b',
      position: 2,
      rewriteLinks: false,
    })
    expect(movePage).not.toHaveBeenCalled()
    expect(result).toEqual({ status: 'moved', preview: noAffectedPreview })
  })

  it('applies with rewriteLinks:true when the user confirms the rewrite', async () => {
    useConfigStore.setState({ enableLinkRefactor: true })
    vi.mocked(previewPageRefactor).mockResolvedValue(preview)
    vi.mocked(confirmPageRefactor).mockResolvedValue(true)
    vi.mocked(applyPageRefactor).mockResolvedValue(null)

    const result = await performTreeCrossParentMove(dragged, {
      parentId: 'section-b',
      index: 2,
    })

    expect(applyPageRefactor).toHaveBeenCalledWith(
      'page-1',
      expect.objectContaining({ rewriteLinks: true }),
    )
    expect(result).toEqual({ status: 'moved', preview })
  })

  it('aborts without calling apply or movePage when the user cancels', async () => {
    useConfigStore.setState({ enableLinkRefactor: true })
    vi.mocked(previewPageRefactor).mockResolvedValue(preview)
    vi.mocked(confirmPageRefactor).mockResolvedValue(null)

    const result = await performTreeCrossParentMove(dragged, {
      parentId: 'section-b',
      index: 2,
    })

    expect(applyPageRefactor).not.toHaveBeenCalled()
    expect(movePage).not.toHaveBeenCalled()
    expect(result).toEqual({ status: 'aborted' })
  })

  it('normalizes the root sentinel to null for the refactor endpoints', async () => {
    useConfigStore.setState({ enableLinkRefactor: true })
    vi.mocked(previewPageRefactor).mockResolvedValue(preview)
    vi.mocked(confirmPageRefactor).mockResolvedValue(false)
    vi.mocked(applyPageRefactor).mockResolvedValue(null)

    await performTreeCrossParentMove(dragged, { parentId: ROOT_ID, index: 0 })

    expect(previewPageRefactor).toHaveBeenCalledWith('page-1', {
      kind: 'move',
      parentId: null,
    })
    expect(applyPageRefactor).toHaveBeenCalledWith(
      'page-1',
      expect.objectContaining({ parentId: null }),
    )
  })
})
