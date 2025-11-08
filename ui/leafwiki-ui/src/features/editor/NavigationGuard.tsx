/* eslint-disable react-hooks/set-state-in-effect */
/**
 * Blocks navigation (internal + external) if `when === true`.
 * Calls `onNavigate(path)` if user confirms internal navigation.
 */

import { UnsavedChangesDialog } from '@/components/UnsavedChangesDialog'
import { useEffect, useState } from 'react'
import { useBlocker } from 'react-router-dom'
import { useExternalUnloadBlocker } from './useExternalUnloadBlocker'

type NavigationGuardProps = {
  when: boolean
  onNavigate: (path: string) => void
}

export default function NavigationGuard({
  when,
  onNavigate,
}: NavigationGuardProps) {
  const blocker = useBlocker(() => when)
  const [showDialog, setShowDialog] = useState(false)
  const [nextPath, setNextPath] = useState<string | null>(null)

  useExternalUnloadBlocker(when)

  useEffect(() => {
    if (blocker.state === 'blocked') {
      const path = blocker.location.pathname + blocker.location.search
      setNextPath(path)
      setShowDialog(true)
    }
  }, [blocker])

  const onCancel = () => {
    setShowDialog(false)
    setNextPath(null)
    if (blocker.reset) {
      blocker.reset()
    }
  }

  const onConfirm = () => {
    setShowDialog(false)
    if (nextPath) {
      if (blocker.proceed) {
        blocker.proceed()
      }
      onNavigate(nextPath)
    }
  }

  return (
    <UnsavedChangesDialog
      open={showDialog}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />
  )
}
