import { Button } from '@/components/ui/button'
import { Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useResyncStore } from '@/stores/resync'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

export default function MaintenanceSettings() {
  const { t } = useTranslation('maintenance')
  const { isLoading, trigger } = useResyncStore()

  useToolbarActions()
  useSetTitle({ title: t('pageTitle') })

  const handleResync = async () => {
    try {
      await trigger()
      toast.success(t('toast.success'))
    } catch {
      toast.error(t('toast.error'))
    }
  }

  return (
    <div className="settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>

      <div className="settings__section">
        <h2 className="settings__section-title">{t('filesystemSync.title')}</h2>
        <p className="settings__section-description">
          {t('filesystemSync.description')}
        </p>
        <div className="settings__actions">
          <Button onClick={handleResync} disabled={isLoading}>
            {isLoading ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="mr-2 h-4 w-4" />
            )}
            {isLoading
              ? t('filesystemSync.buttonLoading')
              : t('filesystemSync.button')}
          </Button>
        </div>
      </div>
    </div>
  )
}
