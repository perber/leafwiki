import {
  applyPageRefactor,
  movePage,
  PageRefactorPreview,
  previewPageRefactor,
} from '@/lib/api/pages'
import { confirmPageRefactor } from '@/features/page/pageRefactorDialogState'
import { useConfigStore } from '@/stores/config'
import { DropResolution, ROOT_ID } from './treeDndUtils'

export type TreeCrossParentMoveResult =
  | { status: 'moved'; preview: PageRefactorPreview | null }
  | { status: 'aborted' }

// Cross-parent tree drag-and-drop moves. When link refactor is disabled this
// is a plain reparent (today's behavior, unchanged). When enabled, it routes
// through the same preview/confirm/apply pipeline the manual Move dialog
// uses, so pages that reference the dragged page get their links rewritten
// too - drag-and-drop otherwise skipped that pipeline entirely and only
// marked those links "broken" in the index.
export async function performTreeCrossParentMove(
  dragged: { id: string; version: string },
  resolution: DropResolution,
): Promise<TreeCrossParentMoveResult> {
  if (!useConfigStore.getState().enableLinkRefactor) {
    await movePage(
      dragged.id,
      dragged.version,
      resolution.parentId,
      resolution.index,
    )
    return { status: 'moved', preview: null }
  }

  const parentId = resolution.parentId === ROOT_ID ? null : resolution.parentId

  const preview = await previewPageRefactor(dragged.id, {
    kind: 'move',
    parentId,
  })
  const rewriteLinks = await confirmPageRefactor(preview, {
    allowSkipRewrite: true,
  })
  if (rewriteLinks === null) {
    return { status: 'aborted' }
  }

  await applyPageRefactor(dragged.id, {
    kind: 'move',
    version: dragged.version,
    parentId,
    position: resolution.index,
    rewriteLinks,
  })

  return { status: 'moved', preview }
}
