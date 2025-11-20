import BaseDialog from '@/components/BaseDialog'
import { Checkbox } from '@/components/ui/checkbox'
import { deletePage } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
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
  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const page = useTreeStore((s) => s.getPageById(pageId))

  const [loading, setLoading] = useState(false)
  const [deleteRecursive, setDeleteRecursive] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})

  if (!page) return null
  const hasChildren = (page.children?.length ?? 0) > 0

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deletePage(pageId, deleteRecursive)
      toast.success('Page deleted successfully')
      navigate(`/${redirectUrl}`)
      await reloadTree()
      return true
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error deleting page')
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_PAGE_CONFIRMATION}
      dialogTitle="Delete Page?"
      dialogDescription="Are you sure you want to delete this page? This action cannot be undone."
      defaultAction="cancel"
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      testidPrefix="delete-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
        loading
      }}
      buttons={[
        {
          label: loading ? 'Deleting...' : 'Delete',
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: loading,
          variant: 'destructive',
        },
      ]}
    >
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
    </BaseDialog>
  )
}
