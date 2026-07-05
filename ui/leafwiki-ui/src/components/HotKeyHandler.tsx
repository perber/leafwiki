// The HotKeyHandler is responsible for handling hotkey events globally in the application.
// It registers event listeners for keydown events and calls the actions defined in the hotkey map.

import {
  getHotkeyComboFromEvent,
  isHotkeyAllowedOnElement,
} from '@/lib/hotkeys'
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
      // Skip events already handled by CodeMirror (e.g. search panel closing),
      // but not in dialog mode: BaseDialog deliberately calls e.preventDefault()
      // in onEscapeKeyDown to prevent Radix from self-closing the dialog, while
      // still relying on HotKeyHandler to dispatch the registered cancelHotkey.
      if (e.defaultPrevented && currentMode !== 'dialog') {
        return
      }

      const target = e.target instanceof HTMLElement ? e.target : null
      if (target?.closest('.cm-search')) {
        return
      }

      const comboString = getHotkeyComboFromEvent(e)

      // Escape bypasses the button/textarea guard below so it always reaches
      // the registered hotkey (e.g. closing a dialog or the editor).
      if (comboString !== 'Escape') {
        // if the focus is on a button or textarea, we don't trigger hotkeys;
        // this allows normal typing and button interactions
        const activeElement = document.activeElement
        if (
          activeElement &&
          (activeElement.tagName === 'BUTTON' ||
            activeElement.tagName === 'INPUT' ||
            activeElement.tagName === 'SELECT' ||
            activeElement.tagName === 'TEXTAREA')
        ) {
          if (!isHotkeyAllowedOnElement(activeElement, comboString)) {
            return
          }
        }
      }

      const registeredKeys = registeredHotkeys[comboString] as
        HotKeyDefinition[] | undefined

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
