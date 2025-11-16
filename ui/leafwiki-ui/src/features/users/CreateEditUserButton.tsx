import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_USER_FORM } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
type CreateEditUserButtonProps = {
  user?: User
}

export function CreateEditUserButton({ user }: CreateEditUserButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)
  return user ? (
    <Button
      size="sm"
      variant="outline"
      onClick={() => openDialog(DIALOG_USER_FORM, { user })}
    >
      Edit
    </Button>
  ) : (
    <Button variant="default" onClick={() => openDialog(DIALOG_USER_FORM)}>
      New User
    </Button>
  )
}
