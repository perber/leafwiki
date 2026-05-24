import { Button } from '@/components/ui/button'
import { CloudUpload, Loader2, TriangleAlert } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { useBackupStore } from '@/stores/backup'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

const POLL_INTERVAL_MS = 5000

function formatDate(value: string | null): string {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export default function BackupSettings() {
  const {
    enabled,
    lastBackupAt,
    lastError,
    isLoading,
    isPolling,
    loadStatus,
    triggerPush,
    stopPolling,
  } = useBackupStore()

  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const lastBackupAtRef = useRef<string | null>(null)

  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: 'Backup Settings' })

  useEffect(() => {
    loadStatus()
  }, [loadStatus])

  // Set up polling after push
  useEffect(() => {
    if (isPolling) {
      lastBackupAtRef.current = lastBackupAt
      pollingRef.current = setInterval(async () => {
        await loadStatus()
      }, POLL_INTERVAL_MS)
    }
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
    }
  }, [isPolling, loadStatus])

  // Stop polling when lastBackupAt advances or an error occurs
  useEffect(() => {
    if (isPolling) {
      const hasNewBackup =
        lastBackupAt !== null &&
        lastBackupAtRef.current !== lastBackupAt
      const hasError = lastError !== ''
      if (hasNewBackup || hasError) {
        stopPolling()
        if (hasError) {
          toast.error(`Backup failed: ${lastError}`)
        } else {
          toast.success('Backup completed successfully')
        }
      }
    }
  }, [lastBackupAt, lastError, isPolling, stopPolling])

  const handlePush = async () => {
    try {
      await triggerPush()
      toast.success('Backup triggered')
    } catch {
      toast.error('Failed to trigger backup')
    }
  }

  return (
    <>
      <div className="settings">
        <h1 className="settings__title">Backup Settings</h1>

        {isLoading && (
          <div className="settings__section">
            <div className="text-muted flex items-center gap-3 text-sm">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading backup status…
            </div>
          </div>
        )}

        {!isLoading && (
          <>
            <div className="settings__section">
              <h2 className="settings__section-title">Git Backup</h2>
              <p className="settings__section-description">
                Automatically pushes wiki changes to the configured remote Git
                repository. Configure the target repository and credentials in
                your server settings.
              </p>

              <div className="settings__preview">
                <span className="settings__preview-label">Status</span>
                {enabled ? (
                  <span className="settings__pill settings__pill-success text-success font-medium">
                    Enabled
                  </span>
                ) : (
                  <span className="settings__role-pill settings__role-pill--default">
                    Disabled
                  </span>
                )}
              </div>

              {enabled && (
                <>
                  <div className="settings__preview">
                    <span className="settings__preview-label">Last backup</span>
                    <span className="text-interface-text text-sm">
                      {isPolling ? (
                        <span className="text-muted flex items-center gap-2">
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          Waiting for backup to complete…
                        </span>
                      ) : (
                        formatDate(lastBackupAt)
                      )}
                    </span>
                  </div>

                  {lastError && (
                    <div className="settings__preview border-error/20 bg-error/5">
                      <span className="settings__preview-label flex items-center gap-1.5">
                        <TriangleAlert className="text-error h-3.5 w-3.5" />
                        Last error
                      </span>
                      <span className="text-error text-sm">{lastError}</span>
                    </div>
                  )}
                </>
              )}

              {!enabled && (
                <p className="settings__hint">
                  Git backup is not enabled. To enable it, configure a remote
                  repository in your server environment settings.
                </p>
              )}
            </div>

            {enabled && (
              <div className="settings__section">
                <h2 className="settings__section-title">Manual Backup</h2>
                <p className="settings__section-description">
                  Trigger an immediate push of all current wiki content to the
                  remote repository without waiting for the next scheduled sync.
                </p>
                <div className="settings__actions">
                  <Button onClick={handlePush} disabled={isPolling}>
                    {isPolling ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <CloudUpload className="mr-2 h-4 w-4" />
                    )}
                    {isPolling ? 'Pushing…' : 'Push now'}
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
