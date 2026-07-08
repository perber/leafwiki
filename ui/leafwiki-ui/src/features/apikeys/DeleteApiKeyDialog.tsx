import BaseDialog from '@/components/BaseDialog'
import { mapApiError } from '@/lib/api/errors'
import { DIALOG_DELETE_API_KEY_CONFIRMATION } from '@/lib/registries'
import { useApiKeyStore } from '@/stores/apikeys'
import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type DeleteApiKeyDialogProps = {
  apiKeyId: string
  apiKeyName: string
}

export function DeleteApiKeyDialog({
  apiKeyId,
  apiKeyName,
}: DeleteApiKeyDialogProps) {
  const { t } = useTranslation('apikeys')
  const { deleteApiKey } = useApiKeyStore()

  const [loading, setLoading] = useState(false)

  const handleRevoke = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await deleteApiKey(apiKeyId)
      toast.success(t('delete.successToast'))
      return true // Close the dialog
    } catch (err) {
      console.error('Error revoking API key:', err)
      const mapped = mapApiError(err, t('delete.errorFallback'))
      toast.error(mapped.message)
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_DELETE_API_KEY_CONFIRMATION}
      dialogTitle={t('delete.title')}
      dialogDescription={t('delete.description')}
      onClose={() => true}
      onConfirm={async (): Promise<boolean> => {
        return await handleRevoke()
      }}
      defaultAction="cancel"
      testidPrefix="delete-api-key-dialog"
      cancelButton={{
        label: t('delete.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? t('delete.confirming') : t('delete.confirm'),
          actionType: 'confirm',
          variant: 'destructive',
          autoFocus: false,
          loading,
          disabled: loading,
        },
      ]}
    >
      <p className="text-muted text-sm">
        <Trans
          i18nKey="delete.body"
          ns="apikeys"
          values={{ name: apiKeyName }}
          components={{ strong: <strong /> }}
        />
      </p>
    </BaseDialog>
  )
}
