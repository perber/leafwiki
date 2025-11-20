import BaseDialog from '@/components/BaseDialog'
import { movePage, PageNode } from '@/lib/api/pages'
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
  const pagePath = getPathById(pageId) || ''
  // get opened route from react router
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

  const handleMove = async () => {
    if (!newParentId || newParentId === parentId) return
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

      toast.success('Page moved successfully')
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error moving page')
    } finally {
      setLoading(false)
    }
  }


  return (
    <BaseDialog
      dialogType={DIALOG_MOVE_PAGE}
      dialogTitle="Move Page"
      dialogDescription="Select a new parent for this page"
      onClose={() => true}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          await handleMove()
          return true
        }
        return false
      }}
      cancelButton={{ label: 'Cancel', variant: 'outline', autoFocus: false }}
      buttons={[{ label: 'Move', actionType: 'confirm', disabled: newParentId === parentId || loading, variant: 'default', autoFocus: true }]}
    >
      <PageSelect pageID={newParentId} onChange={setNewParentId} />
    </BaseDialog>
  )
}
