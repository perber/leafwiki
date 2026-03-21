import BaseDialog from '@/components/BaseDialog'
import { Checkbox } from '@/components/ui/checkbox'
import { deletePage, NODE_KIND_PAGE } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import { useTreeStore } from '@/stores/tree'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export type DeletePageDialogProps = {
  pageId: string
  redirectTo: string
}

export function DeletePageDialog({
  pageId,
  redirectTo,
}: DeletePageDialogProps) {
  const navigate = useNavigate()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const page = useTreeStore((s) => s.getPageById(pageId))

  const [loading, setLoading] = useState(false)
  const [deleteRecursive, setDeleteRecursive] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})

  if (!page) return null
  const hasChildren = (page.children?.length ?? 0) > 0
  const itemLabel = page.kind === NODE_KIND_PAGE ? 'page' : 'section'
  const itemLabelCapitalized = page.kind === NODE_KIND_PAGE ? 'Page' : 'Section'

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deletePage(pageId, deleteRecursive)
      toast.success(`${itemLabelCapitalized} deleted successfully`)
      navigate(redirectTo)
      await reloadTree()
      return true
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, `Error deleting ${itemLabel}`)
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_PAGE_CONFIRMATION}
      dialogTitle={`Delete ${itemLabelCapitalized}?`}
      dialogDescription={`Are you sure you want to delete this ${itemLabel}? This action cannot be undone.`}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      defaultAction="cancel"
      testidPrefix="delete-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
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
        <div className="delete-page-dialog__recursive">
          <label className="delete-page-dialog__recursive-label">
            <Checkbox
              data-testid="delete-page-dialog-recursive-delete-checkbox"
              checked={deleteRecursive}
              onCheckedChange={(val) => setDeleteRecursive(!!val)}
            />{' '}
            {'Also delete all descendant pages and sections'}
          </label>
        </div>
      )}
    </BaseDialog>
  )
}
