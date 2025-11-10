import { FormActions } from '@/components/FormActions'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { movePage, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'

export function MovePageDialog({ pageId }: { pageId: string }) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === 'move')

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
      closeDialog()
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error moving page')
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    closeDialog()
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (!open) {
          closeDialog()
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Move Page</DialogTitle>
        </DialogHeader>
        <DialogDescription>Select a new parent for this page</DialogDescription>
        <PageSelect pageID={newParentId} onChange={setNewParentId} />
        <div className="mt-4 flex justify-end">
          <FormActions
            testidPrefix="move-page-dialog"
            onCancel={handleCancel}
            onSave={handleMove}
            saveLabel={loading ? 'Moving...' : 'Move'}
            disabled={newParentId === parentId || loading}
            loading={loading}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
