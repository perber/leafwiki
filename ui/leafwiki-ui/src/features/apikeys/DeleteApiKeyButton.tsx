import { Button } from '@/components/ui/button'
import type { ApiKey } from '@/lib/api/apikeys'
import { DIALOG_DELETE_API_KEY_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

type DeleteApiKeyButtonProps = {
  apiKey: ApiKey
}

export function DeleteApiKeyButton({ apiKey }: DeleteApiKeyButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  return (
    <Button
      size="sm"
      variant="destructive"
      onClick={() =>
        openDialog(DIALOG_DELETE_API_KEY_CONFIRMATION, {
          apiKeyId: apiKey.id,
          apiKeyName: apiKey.name,
        })
      }
    >
      Revoke
    </Button>
  )
}
