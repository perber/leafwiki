import BaseDialog from '@/components/BaseDialog'
import { mapApiError } from '@/lib/api/errors'
import { DIALOG_DELETE_API_KEY_CONFIRMATION } from '@/lib/registries'
import { useApiKeyStore } from '@/stores/apikeys'
import { useState } from 'react'
import { toast } from 'sonner'

type DeleteApiKeyDialogProps = {
  apiKeyId: string
  apiKeyName: string
}

export function DeleteApiKeyDialog({
  apiKeyId,
  apiKeyName,
}: DeleteApiKeyDialogProps) {
  const { deleteApiKey } = useApiKeyStore()

  const [loading, setLoading] = useState(false)

  const handleRevoke = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deleteApiKey(apiKeyId)
      toast.success('API key revoked successfully')
      return true // Close the dialog
    } catch (err) {
      console.error('Error revoking API key:', err)
      const mapped = mapApiError(
        err,
        'Failed to revoke API key. Please try again.',
      )
      toast.error(mapped.message)
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_API_KEY_CONFIRMATION}
      dialogTitle="Revoke API Key?"
      dialogDescription="Are you sure you want to revoke this API key? This action cannot be undone."
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleRevoke()
      }}
      defaultAction="cancel"
      testidPrefix="delete-api-key-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? 'Revoking...' : 'Revoke',
          actionType: 'confirm',
          variant: 'destructive',
          autoFocus: false,
          loading,
          disabled: loading,
        },
      ]}
    >
      <p className="text-muted text-sm">
        The API key <strong>{apiKeyName}</strong> will immediately stop
        working. Any agent or automation using it will lose access.
      </p>
    </BaseDialog>
  )
}
