import { Button } from '@/components/ui/button'
import { ApiLocalizedError, mapApiError } from '@/lib/api/errors'
import { Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useResyncStore } from '@/stores/resync'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

const PHASE_ORDER = ['tree', 'links', 'tags', 'search'] as const

export default function MaintenanceSettings() {
  const { t } = useTranslation('maintenance')
  const { isLoading, phase, trigger } = useResyncStore()

  useToolbarActions()
  useSetTitle({ title: t('pageTitle') })

  const phaseIndex = phase
    ? PHASE_ORDER.indexOf(phase as (typeof PHASE_ORDER)[number]) + 1
    : 0
  const progressPercent = Math.round((phaseIndex / PHASE_ORDER.length) * 100)

  const handleResync = async () => {
    try {
      await trigger()
      toast.success(t('toast.success'))
    } catch (err) {
      if (
        err instanceof ApiLocalizedError &&
        err.code === 'resync_already_running'
      ) {
        toast.info(t('toast.alreadyRunning'))
      } else {
        toast.error(mapApiError(err, t('toast.error')).message)
      }
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

        {isLoading && (
          <div className="importer__status-banner">
            <div className="importer__status-header">
              <div>
                <div className="settings__preview-label">
                  {t('progress.label')}
                </div>
                <div className="importer__status-title">
                  {phase ? t(`progress.${phase}`) : '…'}
                </div>
              </div>
            </div>
            <div className="importer__status-meta">
              <span>
                {phaseIndex} / {PHASE_ORDER.length}
              </span>
              <span>{progressPercent}%</span>
            </div>
            <div className="importer__progress">
              <div
                className="importer__progress-bar"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
