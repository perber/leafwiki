import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import { useTranslation } from 'react-i18next'
import BaseDialog from './BaseDialog'

type UnsavedChangesDialogProps = {
  onConfirm: () => void
  onCancel: () => void
}

export function UnsavedChangesDialog({
  onConfirm,
  onCancel,
}: UnsavedChangesDialogProps) {
  const { t } = useTranslation('common')
  return (
    <BaseDialog
      dialogTitle={t('unsavedChangesDialog.title')}
      dialogDescription={t('unsavedChangesDialog.description')}
      dialogType={DIALOG_UNSAVED_CHANGES}
      testidPrefix="unsaved-changes-dialog"
      onClose={() => {
        onCancel()
        return true
      }}
      onConfirm={async () => {
        onConfirm()
        return true
      }}
      cancelButton={{
        label: t('unsavedChangesDialog.cancelButton'),
        variant: 'secondary',
        autoFocus: true,
      }}
      buttons={[
        {
          label: t('unsavedChangesDialog.leaveAnywayButton'),
          variant: 'destructive',
          actionType: 'confirm',
          disabled: false,
          loading: false,
        },
      ]}
    ></BaseDialog>
  )
}
