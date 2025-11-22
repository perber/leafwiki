import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Trash2 } from 'lucide-react'
import { useEffect } from 'react'

type DeletePageButtonProps = {
  pageId: string
  redirectUrl: string
}
export function DeletePageButton({
  pageId,
  redirectUrl,
}: DeletePageButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  useEffect(() => {
    const editHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+Delete',
      enabled: true,
      mode: ['view'],
      action: () => {
        openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, { pageId, redirectUrl })
      },
    }

    registerHotkey(editHotkey)

    return () => {
      unregisterHotkey(editHotkey.keyCombo)
    }
  }, [openDialog, pageId, redirectUrl, registerHotkey, unregisterHotkey])

  return (
    <TooltipWrapper label="Delete page (Ctrl + Delete)" side="top" align="center">
      <Button
        className="h-8 w-8 rounded-full shadow-xs"
        variant="destructive"
        size="icon"
        data-testid="delete-page-button"
        onClick={() => {
          openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, { pageId, redirectUrl })
        }}
      >
        <Trash2 />
      </Button>
    </TooltipWrapper>
  )
}
