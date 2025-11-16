import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { DIALOG_DELETE_USER_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useUserStore } from '@/stores/users'
import { AlertDialogDescription } from '@radix-ui/react-alert-dialog'
import { useState } from 'react'
import { toast } from 'sonner'

type DeleteUserDialogProps = {
  userId: string
  username: string
}

export function DeleteUserDialog({ userId, username }: DeleteUserDialogProps) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore(
    (s) => s.dialogType === DIALOG_DELETE_USER_CONFIRMATION,
  )

  const { deleteUser } = useUserStore()

  const [loading, setLoading] = useState(false)

  const handleDelete = async () => {
    setLoading(true)
    try {
      await deleteUser(userId)
      closeDialog()
      toast.success('User deleted successfully')
    } catch (err: { error?: string } | unknown) {
      if (err && typeof err === 'object' && 'error' in err) {
        // Handle specific error message if available
        console.error('Error deleting user:', (err as { error: string }).error)
        toast.error(err.error as string)
      } else {
        // Handle generic error
        console.error('Error deleting user:', err)
        toast.error('Failed to delete user. Please try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={(open) => !open && closeDialog()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            Are you sure you want to delete this user?
          </AlertDialogTitle>
          <AlertDialogDescription>
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <p className="text-sm text-gray-600">
          The user <strong>{username}</strong> will be permanently removed from
          the system.
        </p>

        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            disabled={loading}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-xs"
          >
            {loading ? 'Deleting...' : 'Delete'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
