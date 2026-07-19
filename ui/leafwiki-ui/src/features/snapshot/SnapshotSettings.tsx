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
import { formatBytes } from '@/lib/config'
import { snapshotDownloadUrl } from '@/lib/api/snapshot'
import { ApiLocalizedError, mapApiError } from '@/lib/api/errors'
import {
  AlertTriangle,
  Download,
  HardDriveDownload,
  History,
  Loader2,
  Trash2,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSnapshotStore } from '@/stores/snapshot'
import { useRestoreStore } from '@/stores/restore'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

function formatDate(value: string | null, fallback: string): string {
  if (!value) return fallback
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return fallback
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export default function SnapshotSettings() {
  const { t } = useTranslation('snapshot')
  const { t: tRestore } = useTranslation('restore')
  const {
    enabled,
    retentionCount,
    isRunning,
    lastSnapshotAt,
    lastError,
    lastPruneError,
    snapshots,
    isLoading,
    isListLoading,
    statusError,
    loadStatus,
    loadList,
    triggerNow,
    remove,
  } = useSnapshotStore()
  const {
    isLoading: isRestoring,
    phase: restorePhase,
    isResyncPhase,
    needsIntervention,
    versionWarning,
    trigger: triggerRestore,
    selfRestart,
  } = useRestoreStore()

  const [isTriggering, setIsTriggering] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [restoringId, setRestoringId] = useState<string | null>(null)
  const [isSelfRestarting, setIsSelfRestarting] = useState(false)

  useToolbarActions()
  useSetTitle({ title: t('pageTitle') })

  useEffect(() => {
    loadStatus()
    loadList()
  }, [loadStatus, loadList])

  const handleTrigger = async () => {
    setIsTriggering(true)
    try {
      await triggerNow()
      toast.success(t('toast.backupTriggered'))
    } catch {
      toast.error(t('toast.backupTriggerFailed'))
    } finally {
      setIsTriggering(false)
    }
  }

  const handleDelete = async (id: string) => {
    setDeletingId(id)
    try {
      await remove(id)
      toast.success(t('toast.deleteSuccess'))
    } catch {
      toast.error(t('toast.deleteFailed'))
    } finally {
      setDeletingId(null)
    }
  }

  const handleRestore = async (id: string) => {
    setRestoringId(id)
    try {
      await triggerRestore(id)
      if (useRestoreStore.getState().needsIntervention) {
        return
      }
      toast.success(tRestore('toast.restoreSucceeded'))
      window.location.reload()
    } catch (err) {
      if (
        err instanceof ApiLocalizedError &&
        err.code === 'restore_already_running'
      ) {
        toast.error(tRestore('toast.restoreTriggerFailed'))
      } else {
        toast.error(mapApiError(err, tRestore('toast.restoreFailed')).message)
      }
    } finally {
      setRestoringId(null)
    }
  }

  const handleSelfRestart = async () => {
    setIsSelfRestarting(true)
    await selfRestart()
    // The connection drops as the process replaces itself; give it a moment
    // to come back up before reloading into the fresh instance.
    window.setTimeout(() => window.location.reload(), 5000)
  }

  const restorePhaseLabel = restorePhase
    ? tRestore(`progress.${restorePhase}`, { defaultValue: restorePhase })
    : '…'

  return (
    <div className="settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>
      <p className="settings__section-description">{t('pageDescription')}</p>

      {statusError && (
        <div className="settings__section">
          <p className="text-error text-sm">{statusError}</p>
        </div>
      )}

      {needsIntervention && (
        <div className="settings__section border-error/20 bg-error/5">
          <h2 className="settings__section-title text-error flex items-center gap-2">
            <AlertTriangle className="h-4 w-4" />
            {tRestore('needsInterventionTitle')}
          </h2>
          <p className="settings__section-description">
            {tRestore('needsInterventionDescription')}
          </p>
          <div className="settings__actions">
            <Button
              variant="destructive"
              onClick={handleSelfRestart}
              disabled={isSelfRestarting}
            >
              {isSelfRestarting ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <AlertTriangle className="mr-2 h-4 w-4" />
              )}
              {isSelfRestarting
                ? tRestore('selfRestarting')
                : tRestore('selfRestartButton')}
            </Button>
          </div>
        </div>
      )}

      {isRestoring && (
        <div className="settings__section">
          <div className="importer__status-banner">
            <div className="importer__status-header">
              <div>
                <div className="settings__preview-label">
                  {tRestore('progressLabel')}
                </div>
                <div className="importer__status-title">
                  {restorePhaseLabel}
                </div>
                {isResyncPhase && (
                  <div className="text-muted text-xs">
                    {tRestore('resyncTailLabel')}
                  </div>
                )}
              </div>
            </div>
          </div>
          {versionWarning && (
            <p className="settings__hint text-warning mt-2">
              <strong>{tRestore('versionWarningLabel')}:</strong>{' '}
              {versionWarning}
            </p>
          )}
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
                {t('retentionLabel')}
              </span>
              <span className="text-interface-text text-sm">
                {retentionCount > 0
                  ? t('retentionValue', { count: retentionCount })
                  : t('retentionUnlimited')}
              </span>
            </div>

            <div className="settings__preview">
              <span className="settings__preview-label">
                {t('lastBackupLabel')}
              </span>
              <span className="text-interface-text text-sm">
                {isLoading ? (
                  <span className="text-muted flex items-center gap-2">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    {t('loading')}
                  </span>
                ) : isRunning ? (
                  <span className="text-muted flex items-center gap-2">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    {t('running')}
                  </span>
                ) : (
                  formatDate(lastSnapshotAt, t('never'))
                )}
              </span>
            </div>

            {!isLoading && lastError && (
              <div className="settings__preview border-error/20 bg-error/5">
                <span className="settings__preview-label">
                  {t('lastErrorLabel')}
                </span>
                <span className="text-error text-sm">{lastError}</span>
              </div>
            )}

            {!isLoading && !lastError && lastPruneError && (
              <div className="settings__preview border-warning/20 bg-warning/5">
                <span className="settings__preview-label">
                  {t('lastPruneErrorLabel')}
                </span>
                <span className="text-warning text-sm">{lastPruneError}</span>
              </div>
            )}
          </>
        )}

        {!isLoading && !enabled && (
          <p className="settings__hint">{t('hintDisabled')}</p>
        )}
      </div>

      {!isLoading && enabled && (
        <>
          <div className="settings__section">
            <h2 className="settings__section-title">
              {t('manualSectionTitle')}
            </h2>
            <p className="settings__section-description">
              {t('manualSectionDescription')}
            </p>
            <div className="settings__actions">
              <Button onClick={handleTrigger} disabled={isTriggering}>
                {isTriggering ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <HardDriveDownload className="mr-2 h-4 w-4" />
                )}
                {isTriggering ? t('creating') : t('createNow')}
              </Button>
            </div>
          </div>

          <div className="settings__section">
            <h2 className="settings__section-title">{t('listSectionTitle')}</h2>
            <p className="settings__section-description">
              {t('listSectionDescription')}
            </p>

            {isListLoading ? (
              <Loader2 className="text-muted h-4 w-4 animate-spin" />
            ) : snapshots.length === 0 ? (
              <p className="settings__hint">{t('listEmpty')}</p>
            ) : (
              <div className="flex flex-col gap-2">
                {snapshots.map((snap) => (
                  <div key={snap.id} className="settings__preview">
                    <div className="flex flex-col">
                      <span className="text-interface-text text-sm font-medium">
                        {formatDate(snap.createdAt, snap.id)}
                      </span>
                      <span className="text-muted text-xs">
                        {formatBytes(snap.sizeBytes)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button variant="outline" size="sm" asChild>
                        <a href={snapshotDownloadUrl(snap.id)} download>
                          <Download className="mr-2 h-3.5 w-3.5" />
                          {t('download')}
                        </a>
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={isRestoring || restoringId === snap.id}
                          >
                            {restoringId === snap.id ? (
                              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <History className="mr-2 h-3.5 w-3.5" />
                            )}
                            {tRestore('restoreButton')}
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              {tRestore('restoreConfirmTitle')}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              {tRestore('restoreConfirmDescription')}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>
                              {tRestore('restoreConfirmCancel')}
                            </AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleRestore(snap.id)}
                            >
                              {tRestore('restoreConfirmAction')}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={deletingId === snap.id}
                            className="text-error hover:bg-error/10"
                          >
                            {deletingId === snap.id ? (
                              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Trash2 className="mr-2 h-3.5 w-3.5" />
                            )}
                            {t('delete')}
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              {t('deleteConfirmTitle')}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              {t('deleteConfirmDescription')}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>
                              {t('deleteConfirmCancel')}
                            </AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleDelete(snap.id)}
                            >
                              {t('deleteConfirmAction')}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
