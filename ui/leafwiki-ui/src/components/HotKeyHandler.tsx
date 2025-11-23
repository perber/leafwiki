// The HotKeyHandler is responsible for handling hotkey events globally in the application.
// It registers event listeners for keydown events and calls the actions defined in the hotkey map.

import { useAppMode } from '@/lib/useAppMode'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useCallback, useEffect } from 'react'

export function HotKeyHandler() {
  const appMode = useAppMode()
  const isDialogOpen = useDialogsStore((state) => state.isAnyDialogOpen())
  const registeredHotkeys = useHotKeysStore((state) => state.registeredHotkeys)

  let currentMode = appMode
  if (isDialogOpen) {
    currentMode = 'dialog'
  }

  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const keyCombo = []
      // Construct key combo string like 'Mod+Shift+K'
      // 'Mod' represents 'Ctrl' on Windows/Linux and 'Meta' on Mac
      if (e.ctrlKey || e.metaKey) keyCombo.push('Mod')

      if (e.shiftKey) keyCombo.push('Shift')
      if (e.altKey) keyCombo.push('Alt')

      keyCombo.push(e.key)
      const comboString = keyCombo.join('+')

      // if a button is focused, we should not trigger hotkeys except for Escape
      const activeElement = document.activeElement
      if (
        currentMode !== 'edit' && // only block in non-edit modes
        activeElement &&
        (activeElement.tagName === 'BUTTON' ||
          (activeElement.tagName === 'INPUT' &&
            currentMode !== 'dialog' &&
            currentMode !== 'view') ||
          activeElement.tagName === 'TEXTAREA' ||
          activeElement.getAttribute('contenteditable') === 'true') &&
        comboString !== 'Escape'
      ) {
        // The user is focused on a button or input, do not trigger hotkeys
        // If the Escape key is pressed, we allow it to propagate for dialog closing
        console.debug(
          `Hotkey ${comboString} ignored due to focus on input element`,
        )
        return
      }

      const registeredKeys = registeredHotkeys[comboString] as
        | HotKeyDefinition[]
        | undefined

      if (!registeredKeys || registeredKeys.length === 0) {
        return
      }

      const registeredKey =
        registeredKeys[
          registeredKeys.length - 1
        ] /* get the last registered hotkey */

      if (
        registeredKey &&
        registeredKey.enabled &&
        registeredKey.mode.includes(currentMode)
      ) {
        e.stopPropagation()
        e.preventDefault()
        registeredKey.action()
      }
    },
    [registeredHotkeys, currentMode],
  )

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [onKeyDown])

  return <></>
}
