import { FormActions } from '@/components/FormActions'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { PageNode } from '@/lib/api'
import { useMovePageDialogStore } from '@/stores/movePageDialogStore'
import { JSX, useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

export function MovePageDialog() {
  const currentPath = useLocation().pathname
  const navigate = useNavigate()
  const {
    loading,
    success,
    closeDialog,
    movePage,
    getTree,
    getPathById,
    open,
    parentId,
    pageId,
    path,

  } = useMovePageDialogStore()

  const [tree, setTree] = useState<PageNode | null>(null)
  const [newParentId, setNewParentId] = useState<string>("")
  // Indicates if need to redirect the user after moving the page
  const [redirectUser, setRedirectUser] = useState(false)

  useEffect(() => {
    if (open == true) {
      setTree(getTree())
      setNewParentId(parentId)
    }
  }, [open, parentId])

  // Check if the page to move is open
  useEffect(() => {
    if (pageId) {
      const path = getPathById(pageId)

      if (`${currentPath}` === `/${path}`) {
        setRedirectUser(true)
      } else {
        setRedirectUser(false)
      }
    }
  }, [pageId, path])

  const handleMove = async () => {
    if (!newParentId || newParentId === parentId) return
    movePage(newParentId)
  }

  const onHandleOpenChange = (open: boolean) => {
    if (!open) {
      // Reset the state when the dialog is closed
      setNewParentId('')
      // store
      closeDialog()
    }
  }

  const renderOptions = (node: PageNode, depth = 1): JSX.Element[] => {
    const indent = '—'.repeat(depth)
    const options = [
      <SelectItem key={node.id} value={node.id}>
        {indent} {node.title}
      </SelectItem>,
    ]

    if (node.children?.length) {
      node.children.forEach((child) => {
        options.push(...renderOptions(child, depth + 1))
      })
    }

    return options
  }

  useEffect(() => {
    if (success) {
      if (redirectUser) {
        if (path) {
          navigate(`/${path}`)
        } else {
          navigate('/')
        }
      }

      // Close the dialog and navigate to the new page
      closeDialog()
    }
  }, [success, navigate, path])

  if (!tree) return null

  return (
    <Dialog open={open} onOpenChange={onHandleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Move Page</DialogTitle>
        </DialogHeader>
        <DialogDescription>Select a new parent for this page</DialogDescription>
        <Select value={newParentId} onValueChange={setNewParentId}>
          <SelectTrigger>
            <SelectValue placeholder="Select new parent..." />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key="root" value="root">
              ⬆️ Top Level
            </SelectItem>
            {tree && tree.children.map((child: any) => renderOptions(child))}
          </SelectContent>
        </Select>

        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={() => onHandleOpenChange(false)}
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
