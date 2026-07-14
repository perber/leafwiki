// Hook to provide toolbar actions for the page viewer

import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
} from '@/lib/shortcuts/shortcutCatalog'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useConfigStore } from '@/stores/config'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import {
  Copy,
  Download,
  History,
  Link2,
  Pencil,
  Pin,
  PinOff,
  Printer,
  Trash2,
} from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { type ToolbarButton, useToolbarStore } from '../toolbar/toolbarStore'

export interface ToolbarActionsOptions {
  pageKind?: Page['kind']
  printPage: () => void
  editPage: () => void
  showHistory: () => void
  showPermalink: () => void
  deletePage: () => void
  copyPage: () => void
  downloadMarkdown: () => void
  isPinned: boolean
  onPinToggle: () => void
}

export function useToolbarActions({
  pageKind = NODE_KIND_PAGE,
  printPage,
  editPage,
  showHistory,
  showPermalink,
  deletePage,
  copyPage,
  downloadMarkdown,
  isPinned,
  onPinToggle,
}: ToolbarActionsOptions) {
  const { t } = useTranslation('viewer')
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const enableRevision = useConfigStore((state) => state.enableRevision)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
  const itemLabel = pageKind === NODE_KIND_PAGE ? 'Page' : 'Section'
  const isMacOS =
    typeof navigator !== 'undefined' &&
    /Mac|iPhone|iPad|iPod/.test(navigator.platform)

  useEffect(() => {
    if (appMode !== 'view') {
      setButtons([])
      return
    }

    const readOnlyButtons: ToolbarButton[] = [
      {
        id: 'download-page',
        label: `Download ${itemLabel}`,
        hotkey: '',
        icon: <Download size={18} />,
        action: downloadMarkdown,
      },
    ]

    if (readOnlyMode) {
      setButtons(readOnlyButtons)
      return
    }

    const toolbarButtons: ToolbarButton[] = [
      {
        id: 'edit-page',
        label: `Edit ${itemLabel}`,
        hotkey: getShortcutDisplayLabel('viewer.page.edit', isMacOS),
        icon: <Pencil size={18} />,
        action: editPage,
      },
      {
        id: 'page-permalink',
        label: `Share ${itemLabel}`,
        hotkey: getShortcutDisplayLabel('viewer.page.permalink', isMacOS),
        icon: <Link2 size={18} />,
        variant: 'outline',
        action: showPermalink,
      },
      {
        id: 'print-page',
        label: `Print ${itemLabel}`,
        hotkey: getShortcutDisplayLabel('viewer.page.print', isMacOS),
        icon: <Printer size={18} />,
        action: printPage,
      },
      {
        id: 'copy-page',
        label: `Copy ${itemLabel}`,
        hotkey: getShortcutDisplayLabel('viewer.page.copy', isMacOS),
        icon: <Copy size={18} />,
        variant: 'outline',
        action: copyPage,
      },
      {
        id: 'download-page',
        label: `Download ${itemLabel}`,
        hotkey: '',
        icon: <Download size={18} />,
        variant: 'outline',
        action: downloadMarkdown,
      },
      {
        id: 'pin-page',
        label: isPinned ? t('pinned.unpinPage') : t('pinned.pinPage'),
        hotkey: '',
        icon: isPinned ? <PinOff size={18} /> : <Pin size={18} />,
        variant: 'outline',
        action: onPinToggle,
      },
      {
        id: 'delete-page',
        label: `Delete ${itemLabel}`,
        hotkey: getShortcutDisplayLabel('viewer.page.delete', isMacOS),
        icon: <Trash2 size={18} />,
        variant: 'outline',
        destructive: true,
        className: 'hover:text-red-600 hover:bg-red-100 hover:border-red-300',
        action: deletePage,
      },
    ]

    if (enableRevision) {
      toolbarButtons.splice(2, 0, {
        id: 'page-history',
        label: `${itemLabel} History`,
        hotkey: getShortcutDisplayLabel('viewer.page.history', isMacOS),
        icon: <History size={18} />,
        variant: 'outline',
        action: showHistory,
      })
    }

    setButtons(toolbarButtons)

    const copyHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.copy',
      copyPage,
    )

    const permalinkHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.permalink',
      showPermalink,
    )

    const editHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.edit',
      editPage,
    )

    const printHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.print',
      printPage,
    )

    const historyHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.history',
      showHistory,
    )

    const deleteHotkey: HotKeyDefinition = createHotkeyDefinition(
      'viewer.page.delete',
      deletePage,
    )

    registerHotkey(editHotkey)
    registerHotkey(permalinkHotkey)
    registerHotkey(copyHotkey)
    registerHotkey(printHotkey)
    if (enableRevision) {
      registerHotkey(historyHotkey)
    }
    registerHotkey(deleteHotkey)

    return () => {
      unregisterHotkey(editHotkey.keyCombo)
      unregisterHotkey(permalinkHotkey.keyCombo)
      unregisterHotkey(copyHotkey.keyCombo)
      unregisterHotkey(printHotkey.keyCombo)
      if (enableRevision) {
        unregisterHotkey(historyHotkey.keyCombo)
      }
      unregisterHotkey(deleteHotkey.keyCombo)
    }
  }, [
    appMode,
    readOnlyMode,
    enableRevision,
    setButtons,
    deletePage,
    copyPage,
    downloadMarkdown,
    editPage,
    printPage,
    showHistory,
    showPermalink,
    isPinned,
    onPinToggle,
    registerHotkey,
    unregisterHotkey,
    itemLabel,
    isMacOS,
    t,
  ])
}
