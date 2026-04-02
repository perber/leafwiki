// Hook to provide toolbar actions for the page viewer

import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import type { EditorView } from '@codemirror/view'
import { completionStatus } from '@codemirror/autocomplete'
import { Save, X } from 'lucide-react'
import { useEffect } from 'react'
import { useToolbarStore } from '../toolbar/toolbar'
import { usePageEditorStore } from './pageEditor'

export interface ToolbarActionsOptions {
  savePage: () => void
  closePage: () => void
  formatBold: () => void
  formatItalic: () => void
  insertHeading: (level: 1 | 2 | 3) => void
  getEditorView?: () => EditorView | null
}

// Hook to set up toolbar actions based on app mode and read-only status
export function useToolbarActions({
  savePage,
  closePage,
  formatBold,
  formatItalic,
  insertHeading,
  getEditorView,
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
      keyCombo: 'Mod+KeyS',
      enabled: true,
      mode: ['edit'],
      action: savePage,
    }

    const closeHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['edit'],
      action: () => {
        const view = getEditorView?.()
        if (view && completionStatus(view.state) !== null) {
          return
        }

        closePage()
      },
    }

    const editorShouldHandle = () => {
      const view = getEditorView?.()
      return Boolean(view?.hasFocus)
    }

    const boldHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyB',
      enabled: true,
      mode: ['edit'],
      action: formatBold,
      shouldHandle: editorShouldHandle,
    }

    const italicHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyI',
      enabled: true,
      mode: ['edit'],
      action: formatItalic,
      shouldHandle: editorShouldHandle,
    }

    const heading1Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit1',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(1),
      shouldHandle: editorShouldHandle,
    }

    const heading2Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit2',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(2),
      shouldHandle: editorShouldHandle,
    }

    const heading3Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit3',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(3),
      shouldHandle: editorShouldHandle,
    }

    registerHotkey(saveHotKey)
    registerHotkey(closeHotkey)
    registerHotkey(boldHotkey)
    registerHotkey(italicHotkey)
    registerHotkey(heading1Hotkey)
    registerHotkey(heading2Hotkey)
    registerHotkey(heading3Hotkey)

    return () => {
      unregisterHotkey(saveHotKey.keyCombo)
      unregisterHotkey(closeHotkey.keyCombo)
      unregisterHotkey(boldHotkey.keyCombo)
      unregisterHotkey(italicHotkey.keyCombo)
      unregisterHotkey(heading1Hotkey.keyCombo)
      unregisterHotkey(heading2Hotkey.keyCombo)
      unregisterHotkey(heading3Hotkey.keyCombo)
    }
  }, [
    appMode,
    readOnlyMode,
    setButtons,
    savePage,
    closePage,
    formatBold,
    formatItalic,
    insertHeading,
    getEditorView,
    registerHotkey,
    unregisterHotkey,
    dirty,
  ])
}
