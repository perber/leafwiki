import BaseDialog from '@/components/BaseDialog'
import { mapApiError } from '@/lib/api/errors'
import { DIALOG_DELETE_USER_CONFIRMATION } from '@/lib/registries'
import { useUserStore } from '@/stores/users'
import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type DeleteUserDialogProps = {
  userId: string
  username: string
}

export function DeleteUserDialog({ userId, username }: DeleteUserDialogProps) {
  const { t } = useTranslation('users')
  const { deleteUser } = useUserStore()

  const [loading, setLoading] = useState(false)

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deleteUser(userId)
      toast.success(t('deleteUser.successToast'))
      return true // Close the dialog
    } catch (err) {
      console.error('Error deleting user:', err)
      const mapped = mapApiError(err, t('deleteUser.errorFallback'))
      toast.error(mapped.message)
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_USER_CONFIRMATION}
      dialogTitle={t('deleteUser.title')}
      dialogDescription={t('deleteUser.description')}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      defaultAction="cancel"
      testidPrefix="delete-user-dialog"
      cancelButton={{
        label: t('deleteUser.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? t('deleteUser.deleting') : t('deleteUser.delete'),
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: loading,
        },
      ]}
    >
      <p className="text-muted text-sm">
        <Trans
          i18nKey="deleteUser.body"
          ns="users"
          values={{ username }}
          components={{ strong: <strong /> }}
        />
      </p>
    </BaseDialog>
  )
}
