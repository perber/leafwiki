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
          console.debug(
            `Hotkey ${comboString} ignored due to focus on button or textarea`,
          )
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
