import { FormActions } from '@/components/FormActions'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Checkbox } from '@/components/ui/checkbox'
import { deletePage } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export type DeletePageDialogProps = {
  pageId: string
  redirectUrl: string
}

export function DeletePageDialog({
  pageId,
  redirectUrl,
}: DeletePageDialogProps) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore(
    (s) => s.dialogType === DIALOG_DELETE_PAGE_CONFIRMATION,
  )

  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const page = useTreeStore((s) => s.getPageById(pageId))

  const [loading, setLoading] = useState(false)
  const [deleteRecursive, setDeleteRecursive] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})

  if (!page) return null
  const hasChildren = (page.children?.length ?? 0) > 0

  const handleDelete = async () => {
    setLoading(true)
    try {
      await deletePage(pageId, deleteRecursive)
      toast.success('Page deleted successfully')
      navigate(`/${redirectUrl}`)
      await reloadTree()
      closeDialog()
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error deleting page')
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    closeDialog()
    setLoading(false)
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          setDeleteRecursive(false) // Reset recursive delete option
          closeDialog()
        }
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Page?</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogDescription>
          Are you sure you want to delete this page? This action cannot be
          undone.
        </AlertDialogDescription>

        {hasChildren && (
          <div className="space-y-1 text-sm text-gray-600">
            <label className="flex items-center gap-2">
              <Checkbox
                data-testid="delete-page-dialog-recursive-delete-checkbox"
                checked={deleteRecursive}
                onCheckedChange={(val) => setDeleteRecursive(!!val)}
              />
              Also delete all subpages
            </label>
          </div>
        )}

        <div className="mt-4 flex justify-end">
          <FormActions
            testidPrefix="delete-page-dialog"
            onCancel={handleCancel}
            onSave={handleDelete}
            saveVariant={'destructive'}
            saveLabel={loading ? 'Deleting...' : 'Delete'}
            disabled={loading}
            loading={loading}
            autoFocus="cancel"
          />
        </div>
      </AlertDialogContent>
    </AlertDialog>
  )
}
