import { Button } from '@/components/ui/button'
import { Loader2 } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { useBackupStore } from '@/stores/backup'
import { useSetTitle } from '../viewer/setTitle'

const POLL_INTERVAL_MS = 5000
const DEFAULT_BACKUP_INTERVAL_MINUTES = 60

function formatDate(value: string | null): string {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function getNextBackup(lastBackupAt: string | null): string {
  if (!lastBackupAt) return '—'
  const date = new Date(lastBackupAt)
  if (Number.isNaN(date.getTime())) return '—'
  const nextDate = new Date(date.getTime() + DEFAULT_BACKUP_INTERVAL_MINUTES * 60 * 1000)
  return formatDate(nextDate.toISOString())
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
    <div className="settings">
      <h1 className="settings__title">Backup Settings</h1>

      {isLoading && (
        <div className="settings__section">
          <Loader2 className="h-5 w-5 animate-spin" />
        </div>
      )}

      {!isLoading && (
        <div className="settings__section">
          <h2 className="settings__section-title">Git Backup</h2>

          <div className="settings__field">
            <span>Status:</span>
            <span>{enabled ? 'Enabled' : 'Disabled'}</span>
          </div>

          {enabled && (
            <>
              <div className="settings__field">
                <span>Last backup:</span>
                <span>{formatDate(lastBackupAt)}</span>
              </div>

              <div className="settings__field">
                <span>Next backup:</span>
                <span>{getNextBackup(lastBackupAt)}</span>
              </div>

              <div className="settings__field">
                <span>Last error:</span>
                <span className={lastError ? 'text-destructive' : ''}>
                  {lastError || '—'}
                </span>
              </div>

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
            </>
          )}
        </div>
      )}
    </div>
  )
}