// Hook to provide toolbar actions for the page viewer

import { useAppMode } from '@/lib/useAppMode'
import { isHotkeyAllowedOnElement } from '@/lib/hotkeys'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { closeSearchPanel, searchPanelOpen } from '@codemirror/search'
import type { EditorView } from '@codemirror/view'
import { completionStatus } from '@codemirror/autocomplete'
import { Save, X, Cloud } from 'lucide-react'
import { useEffect } from 'react'
import { useEditorStore } from '@/stores/editor'
import { useToolbarStore } from '../toolbar/toolbarStore'
import { usePageEditorStore } from './pageEditorStore'
import { isDirtyState } from './pageEditorStore'

export interface ToolbarActionsOptions {
  savePage: () => void
  closePage: () => void
  formatBold: () => void
  formatItalic: () => void
  formatInlineCode: () => void
  openLinkDialog: () => void
  insertHeading: (level: 1 | 2 | 3) => void
  getEditorView?: () => EditorView | null
}

// Hook to set up toolbar actions based on app mode and read-only status
export function useToolbarActions({
  savePage,
  closePage,
  formatBold,
  formatItalic,
  formatInlineCode,
  openLinkDialog,
  insertHeading,
  getEditorView,
}: ToolbarActionsOptions) {
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  const dirty = usePageEditorStore(isDirtyState)
  const autoSave = useEditorStore((s) => s.autoSave)
  const toggleAutoSave = useEditorStore((s) => s.toggleAutoSave)

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
      {
        id: 'toggle-auto-save',
        label: 'Auto-save',
        hotkey: '',
        icon: <Cloud size={18} />,
        variant: 'outline',
        active: autoSave,
        className: 'toolbar-button__toggle-auto-save',
        action: toggleAutoSave,
      },
    ])
  }, [
    appMode,
    readOnlyMode,
    setButtons,
    dirty,
    savePage,
    closePage,
    autoSave,
    toggleAutoSave,
  ])

  // Register hotkeys
  useEffect(() => {
    if (readOnlyMode || appMode !== 'edit') {
      return
    }

    const editorCloseShouldHandle = () => {
      const view = getEditorView?.()
      if (!view) return false

      const activeElement = document.activeElement
      if (activeElement instanceof Element) {
        if (isHotkeyAllowedOnElement(activeElement, 'Escape')) {
          return true
        }
      }

      return (
        view.hasFocus ||
        (activeElement instanceof Node && view.dom.contains(activeElement))
      )
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
      shouldHandle: editorCloseShouldHandle,
      action: () => {
        const view = getEditorView?.()
        if (view && completionStatus(view.state) !== null) {
          return
        }

        if (view && searchPanelOpen(view.state)) {
          closeSearchPanel(view)
          return
        }

        closePage()
      },
    }

    const boldHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyB',
      enabled: true,
      mode: ['edit'],
      action: formatBold,
    }

    const italicHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyI',
      enabled: true,
      mode: ['edit'],
      action: formatItalic,
    }

    const heading1Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit1',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(1),
    }

    const heading2Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit2',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(2),
    }

    const heading3Hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+Digit3',
      enabled: true,
      mode: ['edit'],
      action: () => insertHeading(3),
    }

    const inlineCodeHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Backquote',
      enabled: true,
      mode: ['edit'],
      action: formatInlineCode,
    }

    const linkHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyK',
      enabled: true,
      mode: ['edit'],
      action: openLinkDialog,
    }

    registerHotkey(saveHotKey)
    registerHotkey(closeHotkey)
    registerHotkey(boldHotkey)
    registerHotkey(italicHotkey)
    registerHotkey(heading1Hotkey)
    registerHotkey(heading2Hotkey)
    registerHotkey(heading3Hotkey)
    registerHotkey(inlineCodeHotkey)
    registerHotkey(linkHotkey)

    return () => {
      unregisterHotkey(saveHotKey.keyCombo)
      unregisterHotkey(closeHotkey.keyCombo)
      unregisterHotkey(boldHotkey.keyCombo)
      unregisterHotkey(italicHotkey.keyCombo)
      unregisterHotkey(heading1Hotkey.keyCombo)
      unregisterHotkey(heading2Hotkey.keyCombo)
      unregisterHotkey(heading3Hotkey.keyCombo)
      unregisterHotkey(inlineCodeHotkey.keyCombo)
      unregisterHotkey(linkHotkey.keyCombo)
    }
  }, [
    appMode,
    readOnlyMode,
    setButtons,
    savePage,
    closePage,
    formatBold,
    formatItalic,
    formatInlineCode,
    openLinkDialog,
    insertHeading,
    getEditorView,
    registerHotkey,
    unregisterHotkey,
    dirty,
  ])
}
