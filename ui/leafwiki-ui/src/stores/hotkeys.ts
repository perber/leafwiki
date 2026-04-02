// Hotkeys store
// is used to manage global hotkeys in the application

import { normalizeHotkeyCombo } from '@/lib/hotkeys'
import { create } from 'zustand'

export type HotKeyDefinition = {
  keyCombo: string
  enabled: boolean
  mode: string[] // defines in which app modes the hotkey is active
  action: () => void
  shouldHandle?: () => boolean
}

type HotKeysStore = {
  registeredHotkeys: Record<string, HotKeyDefinition[]> // Stacked, last registered has priority
  registerHotkey: (hotKeyDefinition: HotKeyDefinition) => void // Registers a new hotkey and stacks it
  unregisterHotkey: (keyCombo: string) => void // Unregisters the last registered hotkey for the given keyCombo
  getRegisteredHotkeys: () => Record<string, HotKeyDefinition[]> // Returns the current registered hotkeys
}

export const useHotKeysStore = create<HotKeysStore>((set, get) => ({
  registeredHotkeys: {},
  registerHotkey: (hotKeyDefinition: HotKeyDefinition) => {
    const normalizedKeyCombo = normalizeHotkeyCombo(hotKeyDefinition.keyCombo)

    set((state) => {
      const existingHotkeys = state.registeredHotkeys[normalizedKeyCombo] || []
      return {
        registeredHotkeys: {
          ...state.registeredHotkeys,
          [normalizedKeyCombo]: [
            ...existingHotkeys,
            {
              ...hotKeyDefinition,
              keyCombo: normalizedKeyCombo,
            },
          ],
        },
      }
    })
  },
  unregisterHotkey: (keyCombo: string) => {
    const normalizedKeyCombo = normalizeHotkeyCombo(keyCombo)

    set((state) => {
      const existingHotkeys = state.registeredHotkeys[normalizedKeyCombo] || []
      if (existingHotkeys.length === 0) {
        return state // No hotkey to unregister
      }
      const updatedHotkeys = existingHotkeys.slice(0, -1) // Remove the last registered hotkey
      const newRegisteredHotkeys = { ...state.registeredHotkeys }
      if (updatedHotkeys.length === 0) {
        delete newRegisteredHotkeys[normalizedKeyCombo] // Remove the key if no hotkeys left
      } else {
        newRegisteredHotkeys[normalizedKeyCombo] = updatedHotkeys
      }
      return {
        registeredHotkeys: newRegisteredHotkeys,
      }
    })
  },
  getRegisteredHotkeys: () => {
    return get().registeredHotkeys
  },
}))
