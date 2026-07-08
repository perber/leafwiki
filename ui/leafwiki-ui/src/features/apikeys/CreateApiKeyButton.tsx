import { Button } from '@/components/ui/button'
import { DIALOG_API_KEY_FORM } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTranslation } from 'react-i18next'

export function CreateApiKeyButton() {
  const { t } = useTranslation('apikeys')
  const openDialog = useDialogsStore((s) => s.openDialog)
  return (
    <Button variant="default" onClick={() => openDialog(DIALOG_API_KEY_FORM)}>
      {t('create.button')}
    </Button>
  )
}
