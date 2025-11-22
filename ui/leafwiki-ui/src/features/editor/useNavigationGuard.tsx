/* eslint-disable react-hooks/set-state-in-effect */
/**
 * Blocks navigation (internal + external) if `when === true`.
 * Calls `onNavigate(path)` if user confirms internal navigation.
 */

import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useCallback, useEffect } from 'react'
import { useBlocker } from 'react-router-dom'
import { useExternalUnloadBlocker } from './useExternalUnloadBlocker'

type UseNavigationGuardProps = {
  when: boolean
  onNavigate: () => void
}

export default function useNavigationGuard({
  when,
  onNavigate,
}: UseNavigationGuardProps) {
  const blocker = useBlocker(() => when)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const closeDialog = useDialogsStore((state) => state.closeDialog)

  useExternalUnloadBlocker(when)

  // onCancel resets the navigation blocker
  // so the user stays on the current page
  const onCancel = useCallback(() => {
    if (blocker.state === 'blocked' && blocker.reset) {
      blocker.reset()
      console.log('Navigation cancelled', blocker)
    }
  }, [blocker])

  // onConfirm runs the proceed function and then calls onNavigate
  // to perform the navigation action
  const onConfirm = useCallback(() => {
    if (!blocker.location) return

    if (blocker.state !== 'blocked') return

    if (blocker.proceed) {
      blocker.proceed()
    }
    onNavigate()
  }, [blocker, onNavigate])

  // Close the dialog if navigation is unblocked and there is no nextPath
  useEffect(() => {
    if (blocker.state === 'unblocked') {
      closeDialog()
    }
  }, [blocker.state, closeDialog])

  // Open the dialog when there is a blocked navigation
  useEffect(() => {
    if (!blocker.location) return

    if (blocker.state === 'proceeding') return

    openDialog(DIALOG_UNSAVED_CHANGES, {
      onConfirm,
      onCancel,
    })
  }, [blocker.state, blocker.location, onConfirm, onCancel, openDialog])
}
