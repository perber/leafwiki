// Hook to provide toolbar actions for the page viewer

import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Copy, History, Pencil, Printer, Trash2 } from 'lucide-react'
import { useEffect } from 'react'
import { useToolbarStore } from '../toolbar/toolbar'

export interface ToolbarActionsOptions {
  pageKind?: Page['kind']
  printPage: () => void
  editPage: () => void
  deletePage: () => void
  copyPage: () => void
  showHistory: () => void
}

export function useToolbarActions({
  pageKind = NODE_KIND_PAGE,
  printPage,
  editPage,
  deletePage,
  copyPage,
  showHistory,
}: ToolbarActionsOptions) {
  const setButtons = useToolbarStore((state) => state.setButtons)
  const appMode = useAppMode()
  const readOnlyMode = useIsReadOnly()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
  const itemLabel = pageKind === NODE_KIND_PAGE ? 'Page' : 'Section'

  useEffect(() => {
    if (readOnlyMode || appMode !== 'view') {
      setButtons([])
      return
    }

    setButtons([
      {
        id: 'delete-page',
        label: `Delete ${itemLabel}`,
        hotkey: 'Ctrl+Delete',
        icon: <Trash2 size={18} />,
        variant: 'outline',
        className: 'hover:text-red-600 hover:bg-red-100 hover:border-red-300',
        action: deletePage,
      },
      {
        id: 'page-history',
        label: `${itemLabel} History`,
        hotkey: '',
        icon: <History size={18} />,
        variant: 'outline',
        action: showHistory,
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
        id: 'print-page',
        label: `Print ${itemLabel}`,
        hotkey: 'Ctrl+P',
        icon: <Printer size={18} />,
        action: printPage,
      },
      {
        id: 'edit-page',
        label: `Edit ${itemLabel}`,
        hotkey: 'Ctrl+E',
        icon: <Pencil size={18} />,
        action: editPage,
      },
    ])

    const copyHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Shift+S',
      enabled: true,
      mode: ['view'],
      action: copyPage,
    }

    const editHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+e',
      enabled: true,
      mode: ['view'],
      action: editPage,
    }

    const deleteHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Delete',
      enabled: true,
      mode: ['view'],
      action: deletePage,
    }

    registerHotkey(editHotkey)
    registerHotkey(copyHotkey)
    registerHotkey(deleteHotkey)

    return () => {
      unregisterHotkey(editHotkey.keyCombo)
      unregisterHotkey(copyHotkey.keyCombo)
      unregisterHotkey(deleteHotkey.keyCombo)
    }
  }, [
    appMode,
    readOnlyMode,
    setButtons,
    deletePage,
    copyPage,
    showHistory,
    editPage,
    printPage,
    registerHotkey,
    unregisterHotkey,
    itemLabel,
  ])
}
