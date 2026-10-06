import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_CHANGE_USER_PASSWORD } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { useTranslation } from 'react-i18next'

type ChangePasswordButtonProps = {
  user: User
}

export function ChangePasswordButton({ user }: ChangePasswordButtonProps) {
  const { t } = useTranslation('users')
  const openDialog = useDialogsStore((s) => s.openDialog)

  // Own password is changed in the account settings, which asks for the
  // current one; this admin action is for other users only.
  const { user: currentUser } = useSessionStore()
  if (currentUser?.id === user.id) return null

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
      {t('actions.changePassword')}
    </Button>
  )
}
