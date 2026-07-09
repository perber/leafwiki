// Hook to provide toolbar actions for the page viewer

import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
} from '@/lib/shortcuts/shortcutCatalog'
import { useAppMode } from '@/lib/useAppMode'
import { isHotkeyAllowedOnElement } from '@/lib/hotkeys'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { closeSearchPanel, searchPanelOpen } from '@codemirror/search'
import type { EditorView } from '@codemirror/view'
import { completionStatus } from '@codemirror/autocomplete'
import { Save, X, Cloud } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useEditorStore } from '@/stores/editor'
import { useIsMobile } from '@/lib/useIsMobile'
import { type ToolbarButton, useToolbarStore } from '../toolbar/toolbarStore'
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
  const { t } = useTranslation('editor')
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const isMobile = useIsMobile()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  const dirty = usePageEditorStore(isDirtyState)
  const autoSave = useEditorStore((s) => s.autoSave)
  const toggleAutoSave = useEditorStore((s) => s.toggleAutoSave)
  const isMacOS =
    typeof navigator !== 'undefined' &&
    /Mac|iPhone|iPad|iPod/.test(navigator.platform)

  // useEffect to set toolbar buttons
  useEffect(() => {
    if (readOnlyMode || appMode !== 'edit') {
      setButtons([])
      return
    }

    const buttons: ToolbarButton[] = [
      {
        id: 'close-editor',
        label: t('toolbarActions.closeEditor'),
        hotkey: getShortcutDisplayLabel('editor.page.close', isMacOS),
        icon: <X size={18} />,
        action: closePage,
        variant: 'destructive',
        className: 'toolbar-button__close-editor',
      },
      {
        id: 'save-page',
        label: t('toolbarActions.savePage'),
        hotkey: getShortcutDisplayLabel('editor.page.save', isMacOS),
        icon: <Save size={18} />,
        variant: 'default',
        disabled: !dirty,
        className: 'toolbar-button__save-page',
        action: savePage,
      },
    ]

    if (!isMobile) {
      buttons.push({
        id: 'toggle-auto-save',
        label: t('toolbar.autoSave'),
        hotkey: '',
        icon: <Cloud size={18} />,
        variant: 'outline',
        active: autoSave,
        className: 'toolbar-button__toggle-auto-save',
        action: toggleAutoSave,
      })
    }

    setButtons(buttons)
  }, [
    appMode,
    readOnlyMode,
    isMobile,
    setButtons,
    dirty,
    savePage,
    closePage,
    autoSave,
    toggleAutoSave,
    isMacOS,
    t,
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

    const saveHotKey: HotKeyDefinition = createHotkeyDefinition(
      'editor.page.save',
      savePage,
    )

    const closeHotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.page.close',
      () => {
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
      { shouldHandle: editorCloseShouldHandle },
    )

    const boldHotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.format.bold',
      formatBold,
    )

    const italicHotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.format.italic',
      formatItalic,
    )

    const heading1Hotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.heading.one',
      () => insertHeading(1),
    )

    const heading2Hotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.heading.two',
      () => insertHeading(2),
    )

    const heading3Hotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.heading.three',
      () => insertHeading(3),
    )

    const inlineCodeHotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.format.inlineCode',
      formatInlineCode,
    )

    const linkHotkey: HotKeyDefinition = createHotkeyDefinition(
      'editor.link.insert',
      openLinkDialog,
    )

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
    isMacOS,
    t,
  ])
}
