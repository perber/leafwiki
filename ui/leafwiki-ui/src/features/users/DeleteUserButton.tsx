import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_DELETE_USER_CONFIRMATION } from '@/lib/registries'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'

type DeleteUserButtonProps = {
  user: User
}

export function DeleteUserButton({ user }: DeleteUserButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  const { user: currentUser } = useAuthStore()
  if (currentUser?.id === user.id) return null

  return (
    <Button
      size="sm"
      variant="destructive"
      onClick={() =>
        openDialog(DIALOG_DELETE_USER_CONFIRMATION, {
          userId: user.id,
          username: user.username,
        })
      }
    >
      Delete
    </Button>
  )
}
