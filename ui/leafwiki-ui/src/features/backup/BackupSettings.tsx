import { Button } from '@/components/ui/button'
import { CloudUpload, GitMerge, Loader2, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useBackupStore } from '@/stores/backup'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

const POLL_INTERVAL_MS = 5000

function formatDate(value: string | null, fallback: string): string {
  if (!value) return fallback
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return fallback
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export default function BackupSettings() {
  const { t } = useTranslation('backup')
  const {
    enabled,
    lastBackupAt,
    lastError,
    needsIntervention,
    conflictDetails,
    isLoading,
    isPolling,
    pollingFromAt,
    statusError,
    loadStatus,
    triggerPush,
    forcePush,
    stopPolling,
  } = useBackupStore()

  const [isForcePushing, setIsForcePushing] = useState(false)

  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: t('pageTitle') })

  useEffect(() => {
    loadStatus()
  }, [loadStatus])

  useEffect(() => {
    if (!isPolling) return
    const interval = setInterval(() => loadStatus(), POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [isPolling, loadStatus])

  // Stop polling when lastBackupAt advances beyond the pre-push baseline or an error occurs
  useEffect(() => {
    if (!isPolling) return
    const hasNewBackup = lastBackupAt !== null && pollingFromAt !== lastBackupAt
    const hasError = lastError !== ''
    if (hasNewBackup || hasError) {
      stopPolling()
      if (hasError) {
        toast.error(t('toast.backupFailed', { message: lastError }))
      } else {
        toast.success(t('toast.backupCompleted'))
      }
    }
  }, [lastBackupAt, lastError, isPolling, pollingFromAt, stopPolling, t])

  const handlePush = async () => {
    try {
      await triggerPush()
      toast.success(t('toast.backupTriggered'))
    } catch {
      toast.error(t('toast.backupTriggerFailed'))
    }
  }

  const handleForcePush = async () => {
    setIsForcePushing(true)
    try {
      await forcePush()
      toast.success(t('toast.forcePushSuccess'))
    } catch (err) {
      const msg =
        err instanceof Error ? err.message : t('toast.forcePushFailed')
      toast.error(msg)
    } finally {
      setIsForcePushing(false)
    }
  }

  return (
    <div className="settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>
      <p className="settings__section-description">{t('pageDescription')}</p>

      {statusError && (
        <div className="settings__section">
          <p className="text-error text-sm">{statusError}</p>
        </div>
      )}

      <div className="settings__section">
        <h2 className="settings__section-title">{t('sectionTitle')}</h2>
        <p className="settings__section-description">
          {t('sectionDescription')}
        </p>

        <div className="settings__preview">
          <span className="settings__preview-label">{t('statusLabel')}</span>
          {isLoading ? (
            <Loader2 className="text-muted h-4 w-4 animate-spin" />
          ) : enabled ? (
            <span className="settings__pill settings__pill-success text-success font-medium">
              {t('statusEnabled')}
            </span>
          ) : (
            <span className="settings__role-pill settings__role-pill--default">
              {t('statusDisabled')}
            </span>
          )}
        </div>

        {(isLoading || enabled) && (
          <>
            <div className="settings__preview">
              <span className="settings__preview-label">
                {t('lastBackupLabel')}
              </span>
              <span className="text-interface-text text-sm">
                {isLoading || isPolling ? (
                  <span className="text-muted flex items-center gap-2">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    {isPolling ? t('waitingForBackup') : t('loading')}
                  </span>
                ) : (
                  formatDate(lastBackupAt, t('never'))
                )}
              </span>
            </div>

            {!isLoading && lastError && !needsIntervention && (
              <div className="settings__preview border-error/20 bg-error/5">
                <span className="settings__preview-label flex items-center gap-1.5">
                  <TriangleAlert className="text-error h-3.5 w-3.5" />
                  {t('lastErrorLabel')}
                </span>
                <span className="text-error text-sm">{lastError}</span>
              </div>
            )}

            {!isLoading && needsIntervention && (
              <div className="settings__preview border-warning/20 bg-warning/5">
                <span className="settings__preview-label flex items-center gap-1.5">
                  <GitMerge className="text-warning h-3.5 w-3.5" />
                  {t('conflictTitle')}
                </span>
                <div className="flex flex-col gap-2">
                  <span className="text-warning text-sm font-medium">
                    {t('conflictDescription')}
                  </span>
                  <span className="text-muted text-xs">{conflictDetails}</span>
                  <span className="text-muted text-xs">
                    {t('conflictWarning')}
                  </span>
                  <div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleForcePush}
                      disabled={isForcePushing}
                      className="border-warning/40 text-warning hover:bg-warning/10 mt-1"
                    >
                      {isForcePushing ? (
                        <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <GitMerge className="mr-2 h-3.5 w-3.5" />
                      )}
                      {isForcePushing ? t('pushing') : t('forcePushButton')}
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </>
        )}

        {!isLoading && !enabled && (
          <p className="settings__hint">{t('hintDisabled')}</p>
        )}
      </div>

      {!isLoading && enabled && (
        <div className="settings__section">
          <h2 className="settings__section-title">{t('manualSectionTitle')}</h2>
          <p className="settings__section-description">
            {t('manualSectionDescription')}
          </p>
          <div className="settings__actions">
            <Button
              onClick={handlePush}
              disabled={isPolling || needsIntervention}
            >
              {isPolling ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <CloudUpload className="mr-2 h-4 w-4" />
              )}
              {isPolling ? t('pushing') : t('pushNow')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
