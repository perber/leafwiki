/**
 * Blocks navigation (internal + external) if `when === true`.
 * Calls `onNavigate(path)` if user confirms internal navigation.
 */

import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useCallback, useEffect, useRef } from 'react'
import { useBlocker } from 'react-router-dom'
import { useExternalUnloadBlocker } from './useExternalUnloadBlocker'

type UseNavigationGuardProps = {
  when: boolean | (() => boolean)
  onNavigate: () => void
}

export default function useNavigationGuard({
  when,
  onNavigate,
}: UseNavigationGuardProps) {
  const allowNextNavigationRef = useRef(false)
  const shouldBlock = useCallback(() => {
    if (allowNextNavigationRef.current) {
      return false
    }
    return typeof when === 'function' ? when() : when
  }, [when])
  const blocker = useBlocker(shouldBlock)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const closeDialog = useDialogsStore((state) => state.closeDialog)

  useExternalUnloadBlocker(typeof when === 'function' ? when() : when)

  // onCancel resets the navigation blocker
  // so the user stays on the current page
  const onCancel = useCallback(() => {
    if (allowNextNavigationRef.current) {
      return
    }
    allowNextNavigationRef.current = false
    if (blocker.state === 'blocked' && blocker.reset) {
      blocker.reset()
    }
  }, [blocker])

  // onConfirm runs the proceed function and then calls onNavigate
  // to perform the navigation action
  const onConfirm = useCallback(() => {
    if (!blocker.location) return

    if (blocker.state !== 'blocked') return

    allowNextNavigationRef.current = true
    closeDialog()

    if (blocker.proceed) {
      blocker.proceed()
    }
    onNavigate()
  }, [blocker, closeDialog, onNavigate])

  // Close the dialog if navigation is unblocked and there is no nextPath
  useEffect(() => {
    if (blocker.state === 'unblocked') {
      allowNextNavigationRef.current = false
      closeDialog()
    }
  }, [blocker.state, closeDialog])

  useEffect(() => {
    return () => {
      allowNextNavigationRef.current = false
      const dialogsState = useDialogsStore.getState()
      if (dialogsState.dialogType === DIALOG_UNSAVED_CHANGES) {
        dialogsState.closeDialog()
      }
    }
  }, [])

  // Open the dialog when there is a blocked navigation
  useEffect(() => {
    if (!blocker.location) return

    if (blocker.state !== 'blocked') return

    if (allowNextNavigationRef.current) return

    openDialog(DIALOG_UNSAVED_CHANGES, {
      onConfirm,
      onCancel,
    })
  }, [blocker.state, blocker.location, onConfirm, onCancel, openDialog])
}
