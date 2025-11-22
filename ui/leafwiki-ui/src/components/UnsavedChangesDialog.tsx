import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import BaseDialog from './BaseDialog'

type UnsavedChangesDialogProps = {
  onConfirm: () => void
  onCancel: () => void
}

export function UnsavedChangesDialog({
  onConfirm,
  onCancel,
}: UnsavedChangesDialogProps) {
  return (
    <BaseDialog
      dialogTitle="Unsaved changes"
      dialogDescription="You have unsaved changes. Are you sure you want to leave this page? Unsaved data will be lost."
      dialogType={DIALOG_UNSAVED_CHANGES}
      onClose={() => {
        onCancel()
        return true
      }}
      onConfirm={async () => {
        onConfirm()
        return true
      }}
      cancelButton={{
        label: 'Cancel',
        autoFocus: true,
      }}
      buttons={[
        {
          label: 'Leave anyway',
          variant: 'destructive',
          actionType: 'confirm',
          disabled: false,
          loading: false,
        },
      ]}
    ></BaseDialog>
  )
}
