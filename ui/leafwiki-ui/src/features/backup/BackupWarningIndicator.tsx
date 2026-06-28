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
import { useNavigate } from 'react-router-dom'

export function BackupWarningIndicator() {
  const gitBackupEnabled = useConfigStore((s) => s.gitBackupEnabled)
  const user = useSessionStore((s) => s.user)
  const navigate = useNavigate()
  const [needsIntervention, setNeedsIntervention] = useState(false)

  const isAdmin = user?.role === 'admin'
  const isEditorOrAdmin = user?.role === 'admin' || user?.role === 'editor'

  useEffect(() => {
    if (!gitBackupEnabled || !isEditorOrAdmin) return
    fetchBackupAlert()
      .then((r) => setNeedsIntervention(r.needsIntervention))
      .catch(() => {})
  }, [gitBackupEnabled, isEditorOrAdmin])

  if (!gitBackupEnabled || !needsIntervention || !isEditorOrAdmin) return null

  const tooltipText = isAdmin
    ? 'Git backup requires attention. Click to open backup settings.'
    : 'Git backup requires attention. Contact an administrator.'

  const indicator = (
    <button
      type="button"
      aria-label="Backup warning"
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
  )

  return (
    <Tooltip>
      <TooltipTrigger asChild>{indicator}</TooltipTrigger>
      <TooltipContent side="bottom">{tooltipText}</TooltipContent>
    </Tooltip>
  )
}
