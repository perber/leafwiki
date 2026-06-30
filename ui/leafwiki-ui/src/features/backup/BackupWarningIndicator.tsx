import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { fetchBackupAlert } from '@/lib/api/backup'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

const ALERT_POLL_INTERVAL_MS = 30_000

export function BackupWarningIndicator() {
  const { t } = useTranslation('backup')
  const gitBackupEnabled = useConfigStore((s) => s.gitBackupEnabled)
  const user = useSessionStore((s) => s.user)
  const navigate = useNavigate()
  const [needsIntervention, setNeedsIntervention] = useState(false)
  const [hasError, setHasError] = useState(false)

  const isAdmin = user?.role === 'admin'
  const isEditorOrAdmin = user?.role === 'admin' || user?.role === 'editor'

  useEffect(() => {
    if (!gitBackupEnabled || !isEditorOrAdmin) return

    const check = () => {
      fetchBackupAlert()
        .then((r) => {
          setNeedsIntervention(r.needsIntervention)
          setHasError(r.hasError)
        })
        .catch(() => {})
    }

    check()
    const id = setInterval(check, ALERT_POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [gitBackupEnabled, isEditorOrAdmin])

  // Admins see the indicator for any backup error (transient or conflict).
  // Editors only see it for NeedsIntervention (persistent conflict), not transient errors.
  const shouldShowIndicator = isAdmin ? hasError : needsIntervention
  if (!gitBackupEnabled || !shouldShowIndicator || !isEditorOrAdmin) return null

  const tooltipText = isAdmin
    ? needsIntervention
      ? t('tooltip.adminConflict')
      : t('tooltip.adminFailure')
    : needsIntervention
      ? t('tooltip.editorConflict')
      : t('tooltip.editorFailure')

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={t('warningAriaLabel')}
          onClick={isAdmin ? () => navigate('/settings/backup') : undefined}
          className={[
            'flex items-center justify-center rounded p-1 transition-colors',
            isAdmin
              ? 'text-destructive hover:bg-destructive/10 cursor-pointer'
              : 'text-destructive cursor-default',
          ].join(' ')}
        >
          <TriangleAlert className="h-4 w-4" />
        </button>
      </TooltipTrigger>
      <TooltipContent
        side="bottom"
        align="start"
        className="bg-tooltip border-tooltip-border text-tooltip-text z-30 rounded-sm border px-2 py-1 text-xs shadow-sm"
      >
        {tooltipText}
      </TooltipContent>
    </Tooltip>
  )
}
