import BaseDialog from '@/components/BaseDialog'
import {
  applyPageRefactor,
  movePage,
  NODE_KIND_PAGE,
  PageNode,
  PageRefactorPreview,
  previewPageRefactor,
} from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_MOVE_PAGE } from '@/lib/registries'
import { useItemLabels } from '@/lib/useItemLabels'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'
import { refreshAfterPageRefactor } from './pageMutationRefresh'
import { confirmPageRefactor } from './pageRefactorDialogState'

export function MovePageDialog({ pageId }: { pageId: string }) {
  const { t } = useTranslation('page')
  const { t: tCommon } = useTranslation('common')
  const { tree } = useTreeStore()
  const [loading, setLoading] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})
  const page = useTreeStore((s) => s.getPageById(pageId))
  const { item, itemCapitalized } = useItemLabels(page?.kind ?? NODE_KIND_PAGE)
  const enableLinkRefactor = useConfigStore((s) => s.enableLinkRefactor)
  const currentPath = useLocation().pathname
  const navigate = useNavigate()

  const parentId = useMemo(() => {
    const findParent = (node: PageNode): string | null => {
      for (const child of node.children || []) {
        if (child.id === pageId) return node.id
        const found = findParent(child)
        if (found) return found
      }
      return null
    }

    if (!tree) return null
    return findParent(tree)
  }, [tree, pageId])

  const [newParentId, setNewParentId] = useState<string>(parentId || '')

  if (!tree) return null
  if (!parentId) return null
  if (!page) return null

  const getSyntheticMovePreview = (): PageRefactorPreview => {
    const nextParent = newParentId
      ? useTreeStore.getState().getPageById(newParentId)
      : null
    const nextParentPath = nextParent?.path ?? ''
    const normalizedParentPath =
      nextParentPath && nextParentPath !== '/' ? nextParentPath : ''

    return {
      kind: 'move',
      pageId,
      oldPath: page.path,
      newPath: normalizedParentPath
        ? `${normalizedParentPath}/${page.slug}`
        : `/${page.slug}`,
      affectedPages: [],
      counts: {
        affectedPages: 0,
        matchedLinks: 0,
      },
      warnings: [],
    }
  }

  const handleMove = async (): Promise<boolean> => {
    if (!newParentId || newParentId === parentId) return false

    setLoading(true)
    try {
      let preview: PageRefactorPreview

      if (enableLinkRefactor) {
        preview = await previewPageRefactor(pageId, {
          kind: 'move',
          parentId: newParentId,
        })
        const rewriteLinks = await confirmPageRefactor(preview)
        if (rewriteLinks === null) {
          return false
        }

        await applyPageRefactor(pageId, {
          kind: 'move',
          version: page.version,
          parentId: newParentId,
          rewriteLinks,
        })
      } else {
        await movePage(pageId, page.version, newParentId)
        preview = getSyntheticMovePreview()
      }

      await refreshAfterPageRefactor({
        preview,
        currentPath,
        navigate,
      })

      toast.success(t('toast.moved'))
      return true
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, t('toast.moveError', { item }))
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_MOVE_PAGE}
      testidPrefix="move-page-dialog"
      dialogTitle={t('move.title', { item: itemCapitalized })}
      dialogDescription={t('move.description', { item })}
      onClose={() => true}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          return await handleMove()
        }
        return false
      }}
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        autoFocus: false,
        disabled: loading,
      }}
      buttons={[
        {
          label: tCommon('actions.move'),
          actionType: 'confirm',
          disabled: newParentId === parentId || loading,
          variant: 'default',
        },
      ]}
    >
      <PageSelect pageID={newParentId} onChange={setNewParentId} autoFocus />
    </BaseDialog>
  )
}
