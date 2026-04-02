// The HotKeyHandler is responsible for handling hotkey events globally in the application.
// It registers event listeners for keydown events and calls the actions defined in the hotkey map.

import { getHotkeyComboFromEvent } from '@/lib/hotkeys'
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
      const comboString = getHotkeyComboFromEvent(e)

      // Always allow Escape
      if (comboString !== 'Escape') {
        // if the focus in on an button or texarea, we don't trigger hotkeys
        // this allows normal typing and button interactions
        // On input fields, we allow hotkeys to function normally
        const activeElement = document.activeElement
        if (
          activeElement &&
          (activeElement.tagName === 'BUTTON' ||
            activeElement.tagName === 'TEXTAREA')
        ) {
          return
        }
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
        registeredKey.mode.includes(currentMode) &&
        (registeredKey.shouldHandle?.() ?? true)
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
