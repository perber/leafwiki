import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Page } from '@/lib/api/pages'
import { DIALOG_COPY_PAGE } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Copy } from 'lucide-react'
import { useEffect } from 'react'

export function CopyPageButton({ sourcePage }: { sourcePage: Page }) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  useEffect(() => {
    const editHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Shift+S',
      enabled: true,
      mode: ['view'],
      action: () => {
        openDialog(DIALOG_COPY_PAGE, { sourcePage })
      },
    }
    registerHotkey(editHotkey)

    return () => {
      unregisterHotkey(editHotkey.keyCombo)
    }
  }, [openDialog, sourcePage, registerHotkey, unregisterHotkey])

  return (
    <TooltipWrapper
      label="Copy page (Ctrl + Alt + D)"
      side="top"
      align="center"
    >
      <Button
        className="h-8 w-8 rounded-full shadow-xs"
        variant="default"
        size="icon"
        data-testid="copy-page-button"
        onClick={() => {
          openDialog(DIALOG_COPY_PAGE, { sourcePage })
        }}
      >
        <Copy size={20} />
      </Button>
    </TooltipWrapper>
  )
}
