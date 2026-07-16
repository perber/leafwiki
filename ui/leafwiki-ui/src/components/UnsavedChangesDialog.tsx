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
  const { t } = useTranslation('page')

  return (
    <BaseDialog
      dialogTitle={t('unsaved.title')}
      dialogDescription={t('unsaved.description')}
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
        label: t('actions.cancel'),
        variant: 'secondary',
        autoFocus: true,
      }}
      buttons={[
        {
          label: t('actions.leaveAnyway'),
          variant: 'destructive',
          actionType: 'confirm',
          disabled: false,
          loading: false,
        },
      ]}
    ></BaseDialog>
  )
}
