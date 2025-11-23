import { useAppMode } from '@/lib/useAppMode'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useEffect } from 'react'

type PageEditorHotKeysOptions = {
  onSave: () => void
  onCancel: () => void
}

export function usePageEditorHotKeys(options: PageEditorHotKeysOptions) {
  const { onSave, onCancel } = options
  const mode = useAppMode()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  useEffect(() => {
    const saveHotkey = {
      keyCombo: 'Mod+s',
      enabled: true,
      mode: [mode],
      action: () => {
        onSave()
      },
    }

    const cancelHotkey = {
      keyCombo: 'Escape',
      enabled: true,
      mode: [mode],
      action: () => {
        onCancel()
      },
    }

    registerHotkey(saveHotkey)
    registerHotkey(cancelHotkey)

    return () => {
      unregisterHotkey(saveHotkey.keyCombo)
      unregisterHotkey(cancelHotkey.keyCombo)
    }
  }, [onSave, onCancel, registerHotkey, unregisterHotkey, mode])
}
