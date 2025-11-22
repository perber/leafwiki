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
  console.log('registered hotkeys:', registeredHotkeys)
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      console.log('key down', e.key)
      const keyCombo = []
      // Construct key combo string like 'Mod+Shift+K'
      // 'Mod' represents 'Ctrl' on Windows/Linux and 'Meta' on Mac
      if (e.ctrlKey || e.metaKey) keyCombo.push('Mod')

      if (e.shiftKey) keyCombo.push('Shift')
      if (e.altKey) keyCombo.push('Alt')

      keyCombo.push(e.key)
      const comboString = keyCombo.join('+')
      console.log('constructed key combo:', comboString)

      const registredKey = registeredHotkeys[comboString] as
        | HotKeyDefinition
        | undefined
      if (
        registredKey &&
        registredKey.enabled &&
        registredKey.mode.includes(currentMode)
      ) {
        e.preventDefault()
        console.log('hotkey matched:', comboString)
        registredKey.action()
      }
    },
    [registeredHotkeys, currentMode],
  )

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    console.log('added keydown listener')
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      console.log('removed keydown listener')
    }
  }, [onKeyDown])

  return <></>
}
