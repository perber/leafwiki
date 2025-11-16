import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_CHANGE_USER_PASSWORD } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

type ChangePasswordButtonProps = {
  user: User
}

export function ChangePasswordButton({ user }: ChangePasswordButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)
  return (
    <Button
      size="sm"
      variant="secondary"
      onClick={() =>
        openDialog(DIALOG_CHANGE_USER_PASSWORD, {
          userId: user.id,
          username: user.username,
        })
      }
    >
      Change Password
    </Button>
  )
}
