import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Loader2 } from 'lucide-react'
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

  // Stop polling when lastBackupAt advances
  useEffect(() => {
    if (isPolling && lastBackupAtRef.current !== null && lastBackupAt !== null) {
      if (lastBackupAtRef.current !== lastBackupAt) {
        stopPolling()
        toast.success('Backup completed successfully')
      }
    }
  }, [lastBackupAt, isPolling, stopPolling])

  const handlePush = async () => {
    try {
      await triggerPush()
      toast.success('Backup triggered')
    } catch (err) {
      toast.error('Failed to trigger backup')
    }
  }

  return (
    <>
      <div className="settings">
        <h1 className="settings__title">Backup Settings</h1>

        {isLoading && (
          <div className="settings__section">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        )}

        {!isLoading && (
          <>
            <div className="settings__section">
              <h2 className="settings__section-title">Git Backup</h2>
              <p className="settings__section-description">
                Automatically pushes changes to the configured remote repository.
              </p>

              <div className="settings__field">
                <Label>Status</Label>
                <span>{enabled ? 'Enabled' : 'Disabled'}</span>
              </div>

              {!enabled && (
                <p className="settings__hint">
                  To enable Git backup, set the environment variable{' '}
                  <code className="px-1 py-0.5 rounded bg-muted text-foreground">LEAFWIKI_GIT_BACKUP=true</code>{' '}
                  and configure{' '}
                  <code className="px-1 py-0.5 rounded bg-muted text-foreground">LEAFWIKI_GIT_BACKUP_REMOTE</code>.
                </p>
              )}

              {enabled && (
                <>
                  <div className="settings__field">
                    <Label>Last backup</Label>
                    <span>{formatDate(lastBackupAt)}</span>
                  </div>

                  <div className="settings__field">
                    <Label>Last error</Label>
                    <span className={lastError ? 'text-destructive' : ''}>
                      {lastError || '—'}
                    </span>
                  </div>
                </>
              )}
            </div>

            {enabled && (
              <div className="settings__actions">
                <Button
                  onClick={handlePush}
                  disabled={isPolling}
                >
                  {isPolling ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : null}
                  Push now
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
