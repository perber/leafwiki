import { Button } from '@/components/ui/button'
import { DIALOG_API_KEY_FORM } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

export function CreateApiKeyButton() {
  const openDialog = useDialogsStore((s) => s.openDialog)
  return (
    <Button variant="default" onClick={() => openDialog(DIALOG_API_KEY_FORM)}>
      New API Key
    </Button>
  )
}
