import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_USER_FORM } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTranslation } from 'react-i18next'

type CreateEditUserButtonProps = {
  user?: User
}

export function CreateEditUserButton({ user }: CreateEditUserButtonProps) {
  const { t } = useTranslation('users')
  const openDialog = useDialogsStore((s) => s.openDialog)
  return user ? (
    <Button
      size="sm"
      variant="outline"
      onClick={() => openDialog(DIALOG_USER_FORM, { user })}
    >
      {t('actions.edit')}
    </Button>
  ) : (
    <Button variant="default" onClick={() => openDialog(DIALOG_USER_FORM)}>
      {t('newUser')}
    </Button>
  )
}
