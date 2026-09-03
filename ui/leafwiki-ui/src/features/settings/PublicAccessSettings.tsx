import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { mapApiError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { Globe, Loader2, Lock } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'

export default function PublicAccessSettings() {
  const { t } = useTranslation('publicAccess')
  useSetTitle({ title: t('pageTitle') })

  const publicAccess = useConfigStore((s) => s.publicAccess)
  const envManaged = useConfigStore((s) => s.publicAccessEnvManaged)
  const setPublicAccess = useConfigStore((s) => s.setPublicAccess)

  const [saving, setSaving] = useState(false)

  const apply = async (enabled: boolean) => {
    setSaving(true)
    try {
      await setPublicAccess(enabled)
      toast.success(enabled ? t('enabledToast') : t('disabledToast'))
    } catch (err) {
      toast.error(mapApiError(err, t('updateError')).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div data-testid="public-access-settings">
      <h2 className="settings__section-title">{t('sectionTitle')}</h2>
      <p className="settings__section-description">{t('sectionDescription')}</p>

      <div className="settings__field">
        <p className="settings__status-line">
          {publicAccess ? (
            <Globe className="settings__status-icon" aria-hidden />
          ) : (
            <Lock className="settings__status-icon" aria-hidden />
          )}
          {publicAccess ? t('statusOn') : t('statusOff')}
        </p>

        {envManaged ? (
          <div
            className="settings__notice"
            data-testid="public-access-env-managed"
          >
            <strong>{t('envManagedTitle')}</strong>
            <span>{t('envManagedDescription')}</span>
          </div>
        ) : publicAccess ? (
          <Button
            variant="outline"
            disabled={saving}
            onClick={() => void apply(false)}
            data-testid="public-access-disable"
          >
            {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t('disableButton')}
          </Button>
        ) : (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button disabled={saving} data-testid="public-access-enable">
                {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t('enableButton')}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t('confirmTitle')}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t('confirmDescription')}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('cancel')}</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() => void apply(true)}
                  data-testid="public-access-confirm"
                >
                  {t('confirmAction')}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}
      </div>
    </div>
  )
}
