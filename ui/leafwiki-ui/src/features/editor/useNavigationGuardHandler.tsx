/* eslint-disable react-hooks/set-state-in-effect */
/**
 * Blocks navigation (internal + external) if `when === true`.
 * Calls `onNavigate(path)` if user confirms internal navigation.
 */

import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useCallback, useEffect, useState } from 'react'
import { useBlocker } from 'react-router-dom'
import { useExternalUnloadBlocker } from './useExternalUnloadBlocker'

type UseNavigationGuardHandlerProps = {
  when: boolean
  onNavigate: (path: string) => void
}

export default function useNavigationGuardHandler({
  when,
  onNavigate,
}: UseNavigationGuardHandlerProps) {
  const blocker = useBlocker(() => when)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const [nextPath, setNextPath] = useState<string | null>(null)

  useExternalUnloadBlocker(when)

  const onCancel = useCallback(() => {
    setNextPath(null)
    if (blocker.reset) {
      blocker.reset()
    }
  }, [blocker])

  const onConfirm = useCallback(() => {
    if (nextPath) {
      if (blocker.proceed) {
        blocker.proceed()
      }
      onNavigate(nextPath)
    }
  }, [nextPath, blocker, onNavigate])

  useEffect(() => {
    if (blocker.state === 'blocked') {
      const path = blocker.location.pathname + blocker.location.search
      setNextPath(path)
      openDialog(DIALOG_UNSAVED_CHANGES, {
        onConfirm,
        onCancel,
      })
    }
  }, [blocker, onCancel, onConfirm, openDialog])
}
