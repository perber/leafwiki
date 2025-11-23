import { PageToolbarButton } from '@/components/PageToolbarButton'
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
    <PageToolbarButton
      label="Copy page"
      hotkey="Ctrl + Shift + S"
      onClick={() => {
        openDialog(DIALOG_COPY_PAGE, { sourcePage })
      }}
      icon={<Copy size={20} />}
    />
  )
}
