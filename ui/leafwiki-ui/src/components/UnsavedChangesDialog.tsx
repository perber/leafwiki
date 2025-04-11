import { FormActions } from '@/components/FormActions'
import {
    AlertDialog,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'
  
  type UnsavedChangesDialogProps = {
    open: boolean
    onConfirm: () => void
    onCancel: () => void
    loading?: boolean
  }
  
  export function UnsavedChangesDialog({
    open,
    onConfirm,
    onCancel,
    loading = false,
  }: UnsavedChangesDialogProps) {
    return (
      <AlertDialog open={open} onOpenChange={(val) => !val && onCancel()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Unsave changes?</AlertDialogTitle>
          </AlertDialogHeader>
          <AlertDialogDescription>
            You have unsaved changes. Are you sure you want to leave this page? Unsaved data will be lost.
          </AlertDialogDescription>
  
          <div className="mt-4 flex justify-end">
            <FormActions
              onCancel={onCancel}
              onSave={onConfirm}
              saveLabel="Leave anyway"
              disabled={loading}
              loading={loading}
            />
          </div>
        </AlertDialogContent>
      </AlertDialog>
    )
  }
  