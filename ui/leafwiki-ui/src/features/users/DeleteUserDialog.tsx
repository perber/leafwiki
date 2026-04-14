import BaseDialog from '@/components/BaseDialog'
import { mapApiError } from '@/lib/api/errors'
import { DIALOG_DELETE_USER_CONFIRMATION } from '@/lib/registries'
import { useUserStore } from '@/stores/users'
import { useState } from 'react'
import { toast } from 'sonner'

type DeleteUserDialogProps = {
  userId: string
  username: string
}

export function DeleteUserDialog({ userId, username }: DeleteUserDialogProps) {
  const { deleteUser } = useUserStore()

  const [loading, setLoading] = useState(false)

  const handleDelete = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deleteUser(userId)
      toast.success('User deleted successfully')
      return true // Close the dialog
    } catch (err) {
      console.error('Error deleting user:', err)
      const mapped = mapApiError(
        err,
        'Failed to delete user. Please try again.',
      )
      toast.error(
        mapped.detail ? `${mapped.message}: ${mapped.detail}` : mapped.message,
      )
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_USER_CONFIRMATION}
      dialogTitle="Delete User?"
      dialogDescription="Are you sure you want to delete this user? This action cannot be undone."
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleDelete()
      }}
      defaultAction="cancel"
      testidPrefix="delete-user-dialog"
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
        },
      ]}
    >
      <p className="text-muted text-sm">
        The user <strong>{username}</strong> will be permanently removed from
        the system.
      </p>
    </BaseDialog>
  )
}
