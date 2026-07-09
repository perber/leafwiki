import BaseDialog from '@/components/BaseDialog'
import { mapApiError } from '@/lib/api/errors'
import { DIALOG_DELETE_USER_CONFIRMATION } from '@/lib/registries'
import { useUserStore } from '@/stores/users'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type DeleteUserDialogProps = {
  userId: string
  username: string
}

export function DeleteUserDialog({ userId, username }: DeleteUserDialogProps) {
  const { t } = useTranslation('users')
  const { t: tCommon } = useTranslation('common')
  const { deleteUser } = useUserStore()

  const [loading, setLoading] = useState(false)

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deleteUser(userId)
      toast.success(t('toast.deleted'))
      return true
    } catch (err) {
      console.error('Error deleting user:', err)
      const mapped = mapApiError(err, t('toast.deleteFailed'))
      toast.error(mapped.message)
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_USER_CONFIRMATION}
      dialogTitle={t('deleteUserTitle')}
      dialogDescription={t('deleteUserDescription')}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      defaultAction="cancel"
      testidPrefix="delete-user-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? tCommon('actions.deleting') : t('actions.delete'),
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: loading,
        },
      ]}
    >
      <p className="text-muted text-sm">
        {t('deleteConfirmBody', { username })}
      </p>
    </BaseDialog>
  )
}
