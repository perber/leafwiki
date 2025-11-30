// Hook to provide toolbar actions for the page viewer

import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Save, X } from 'lucide-react'
import { useEffect } from 'react'
import { useToolbarStore } from '../toolbar/toolbar'
import { usePageEditorStore } from './pageEditor'

export interface ToolbarActionsOptions {
  savePage: () => void
  closePage: () => void
}

// Hook to set up toolbar actions based on app mode and read-only status
export function useToolbarActions({
  savePage,
  closePage,
}: ToolbarActionsOptions) {
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  const dirty = usePageEditorStore((s) => {
    const { page, title, slug, content } = s
    if (!page) return false
    return (
      page.title !== title || page.slug !== slug || page.content !== content
    )
  })

  // useEffect to set toolbar buttons
  useEffect(() => {
    if (readOnlyMode || appMode !== 'edit') {
      setButtons([])
      return
    }

    setButtons([
      {
        id: 'close-editor',
        label: 'Close Editor',
        hotkey: 'Esc',
        icon: <X size={18} />,
        action: closePage,
        variant: 'destructive',
        className: 'toolbar-button__close-editor',
      },
      {
        id: 'save-page',
        label: 'Save Page',
        hotkey: 'Ctrl+S',
        icon: <Save size={18} />,
        variant: 'default',
        disabled: !dirty,
        className: 'toolbar-button__save-page',
        action: savePage,
      },
    ])
  }, [appMode, readOnlyMode, setButtons, dirty, savePage, closePage])

  // Register hotkeys
  useEffect(() => {
    if (readOnlyMode || appMode !== 'edit') {
      return
    }
    const saveHotKey: HotKeyDefinition = {
      keyCombo: 'Mod+s',
      enabled: true,
      mode: ['edit'],
      action: savePage,
    }

    const closeHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['edit'],
      action: closePage,
    }

    registerHotkey(saveHotKey)
    registerHotkey(closeHotkey)

    return () => {
      unregisterHotkey(saveHotKey.keyCombo)
      unregisterHotkey(closeHotkey.keyCombo)
    }
  }, [
    appMode,
    readOnlyMode,
    setButtons,
    savePage,
    closePage,
    registerHotkey,
    unregisterHotkey,
    dirty,
  ])
}
