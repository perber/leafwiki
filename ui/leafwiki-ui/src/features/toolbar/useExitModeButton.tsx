// Registers a single outline back-arrow toolbar button plus a matching
// hotkey for "leave this app mode" flows (history, settings, ...), and
// cleans both up on unmount. Extracted out of PageHistoryPage's original
// hand-rolled version so settings mode can reuse the exact same pattern.

import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
  type ShortcutId,
} from '@/lib/shortcuts/shortcutCatalog'
import { type HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { ArrowLeft } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useToolbarStore } from './toolbarStore'

const isMacOS =
  typeof navigator !== 'undefined' &&
  /Mac|iPhone|iPad|iPod/.test(navigator.platform)

export type UseExitModeButtonOptions = {
  id: string
  labelKey: string
  ns: string
  hotkeyId: ShortcutId
  onExit: () => void
}

export function useExitModeButton({
  id,
  labelKey,
  ns,
  hotkeyId,
  onExit,
}: UseExitModeButtonOptions) {
  const { t } = useTranslation(ns)
  const setToolbarButtons = useToolbarStore((state) => state.setButtons)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)

  useEffect(() => {
    setToolbarButtons([
      {
        id,
        label: t(labelKey),
        hotkey: getShortcutDisplayLabel(hotkeyId, isMacOS),
        icon: <ArrowLeft size={18} />,
        action: onExit,
        variant: 'outline',
      },
    ])

    const hotkey: HotKeyDefinition = createHotkeyDefinition(hotkeyId, onExit)
    registerHotkey(hotkey)

    return () => {
      setToolbarButtons([])
      unregisterHotkey(hotkey.keyCombo)
    }
  }, [
    id,
    labelKey,
    hotkeyId,
    onExit,
    t,
    setToolbarButtons,
    registerHotkey,
    unregisterHotkey,
  ])
}
