// Hook to provide toolbar actions for the page viewer

import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useConfigStore } from '@/stores/config'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Copy, History, Link2, Pencil, Printer, Trash2 } from 'lucide-react'
import { useEffect } from 'react'
import { type ToolbarButton, useToolbarStore } from '../toolbar/toolbar'

export interface ToolbarActionsOptions {
  pageKind?: Page['kind']
  printPage: () => void
  editPage: () => void
  showHistory: () => void
  showPermalink: () => void
  deletePage: () => void
  copyPage: () => void
}

export function useToolbarActions({
  pageKind = NODE_KIND_PAGE,
  printPage,
  editPage,
  showHistory,
  showPermalink,
  deletePage,
  copyPage,
}: ToolbarActionsOptions) {
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const enableRevision = useConfigStore((state) => state.enableRevision)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
  const itemLabel = pageKind === NODE_KIND_PAGE ? 'Page' : 'Section'

  useEffect(() => {
    if (readOnlyMode || appMode !== 'view') {
      setButtons([])
      return
    }

    const toolbarButtons: ToolbarButton[] = [
      {
        id: 'edit-page',
        label: `Edit ${itemLabel}`,
        hotkey: 'Ctrl+E',
        icon: <Pencil size={18} />,
        action: editPage,
      },
      {
        id: 'print-page',
        label: `Print ${itemLabel}`,
        hotkey: 'Ctrl+P',
        icon: <Printer size={18} />,
        action: printPage,
      },
      {
        id: 'page-permalink',
        label: `Share ${itemLabel}`,
        hotkey: 'Ctrl+Shift+L',
        icon: <Link2 size={18} />,
        variant: 'outline',
        action: showPermalink,
      },
      {
        id: 'copy-page',
        label: `Copy ${itemLabel}`,
        hotkey: 'Ctrl+Shift+S',
        icon: <Copy size={18} />,
        variant: 'outline',
        action: copyPage,
      },
      {
        id: 'delete-page',
        label: `Delete ${itemLabel}`,
        hotkey: 'Ctrl+Delete',
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
        hotkey: 'Ctrl+H',
        icon: <History size={18} />,
        variant: 'outline',
        action: showHistory,
      })
    }

    setButtons(toolbarButtons)

    const copyHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Shift+KeyS',
      enabled: true,
      mode: ['view'],
      action: copyPage,
    }

    const permalinkHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Shift+KeyL',
      enabled: true,
      mode: ['view'],
      action: showPermalink,
    }

    const editHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyE',
      enabled: true,
      mode: ['view'],
      action: editPage,
    }

    const printHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyP',
      enabled: true,
      mode: ['view'],
      action: printPage,
    }

    const historyHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+KeyH',
      enabled: true,
      mode: ['view'],
      action: showHistory,
    }

    const deleteHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Delete',
      enabled: true,
      mode: ['view'],
      action: deletePage,
    }

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
    editPage,
    printPage,
    showHistory,
    showPermalink,
    registerHotkey,
    unregisterHotkey,
    itemLabel,
  ])
}
