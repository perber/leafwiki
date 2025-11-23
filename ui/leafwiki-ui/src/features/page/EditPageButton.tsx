import { PageToolbarButton } from '@/components/PageToolbarButton'
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
    <PageToolbarButton
      label="Edit page"
      hotkey="Ctrl+E"
      onClick={() => navigate(buildEditUrl(path))}
      icon={<Pencil size={20} />}
    />
  )
}
