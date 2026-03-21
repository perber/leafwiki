import BaseDialog from '@/components/BaseDialog'
import { movePage, NODE_KIND_PAGE, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_MOVE_PAGE } from '@/lib/registries'
import { useTreeStore } from '@/stores/tree'
import { useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'

export function MovePageDialog({ pageId }: { pageId: string }) {
  const { tree, reloadTree } = useTreeStore()
  const [loading, setLoading] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})
  const getPathById = useTreeStore((s) => s.getPathById)
  const page = useTreeStore((s) => s.getPageById(pageId))
  const pagePath = getPathById(pageId) || ''
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

  const itemLabel = page.kind === NODE_KIND_PAGE ? 'page' : 'section'
  const itemLabelCapitalized = page.kind === NODE_KIND_PAGE ? 'Page' : 'Section'

  const handleMove = async (): Promise<boolean> => {
    if (!newParentId || newParentId === parentId) return false

    setLoading(true)
    try {
      await movePage(pageId, newParentId)
      if (`${currentPath}` === `/${pagePath}`) {
        await reloadTree()
        const newPath = getPathById(pageId) || ''
        if (newPath) {
          navigate(`/${newPath}`)
        } else {
          navigate('/')
        }
      } else {
        await reloadTree()
      }

      toast.success(`${itemLabelCapitalized} moved successfully`)
      return true
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, `Error moving ${itemLabel}`)
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_MOVE_PAGE}
      testidPrefix="move-page-dialog"
      dialogTitle={`Move ${itemLabelCapitalized}`}
      dialogDescription={`Select a new parent for this ${itemLabel}`}
      onClose={() => true}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          return await handleMove()
        }
        return false
      }}
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        autoFocus: false,
        disabled: loading,
      }}
      buttons={[
        {
          label: 'Move',
          actionType: 'confirm',
          disabled: newParentId === parentId || loading,
          variant: 'default',
          autoFocus: true,
        },
      ]}
    >
      <PageSelect pageID={newParentId} onChange={setNewParentId} />
    </BaseDialog>
  )
}
