import { Button } from '@/components/ui/button'
import { DIALOG_PAGE_QUICK_SWITCHER } from '@/lib/registries'
import { useAppMode } from '@/lib/useAppMode'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { FileSearch } from 'lucide-react'
import { useEffect } from 'react'

const isMacOS =
  typeof navigator !== 'undefined' &&
  /Mac|iPhone|iPad|iPod/.test(navigator.platform)
const quickSwitcherHotkeyLabel = isMacOS ? 'Cmd+Option+P' : 'Ctrl+Alt+P'

export function PageQuickSwitcherTrigger() {
  const appMode = useAppMode()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)

  useEffect(() => {
    const hotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Alt+KeyP',
      enabled: true,
      mode: ['view', 'history'],
      action: () => openDialog(DIALOG_PAGE_QUICK_SWITCHER),
    }

    registerHotkey(hotkey)
    return () => unregisterHotkey(hotkey.keyCombo)
  }, [openDialog, registerHotkey, unregisterHotkey])

  if (appMode !== 'view' && appMode !== 'history') {
    return null
  }

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={() => openDialog(DIALOG_PAGE_QUICK_SWITCHER)}
      aria-label="Go to page"
      title={`Go to page (${quickSwitcherHotkeyLabel})`}
      className="max-md:px-2"
    >
      <FileSearch size={16} />
      <span className="max-md:hidden">Go to page</span>
      <span className="text-muted-foreground ml-1 hidden text-xs md:inline">
        {quickSwitcherHotkeyLabel}
      </span>
    </Button>
  )
}
