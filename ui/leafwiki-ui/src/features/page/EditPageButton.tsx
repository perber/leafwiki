import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { buildEditUrl } from '@/lib/urlUtil'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Pencil } from 'lucide-react'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

// This component needs refactoring the keyhandling should be moved to a hook similar to usePageEditorHotKeys
// The provided action should be called inside the PageViewer component
export function EditPageButton({ path }: { path: string }) {
  const navigate = useNavigate()
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  useEffect(() => {
    const editHotkey: HotKeyDefinition = {
      keyCombo: 'Mod+e',
      enabled: true,
      mode: ['view'],
      action: () => {
        navigate(buildEditUrl(path))
      },
    }

    registerHotkey(editHotkey)

    return () => {
      unregisterHotkey(editHotkey.keyCombo)
    }
  }, [navigate, path, registerHotkey, unregisterHotkey])

  return (
    <TooltipWrapper label="Edit page (Ctrl + e)" side="top" align="center">
      <Button
        data-testid="edit-page-button"
        className="h-8 w-8 rounded-full shadow-xs"
        variant="default"
        size="icon"
        onClick={() => navigate(buildEditUrl(path))}
      >
        <Pencil size={20} />
      </Button>
    </TooltipWrapper>
  )
}
