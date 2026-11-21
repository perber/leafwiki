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
  unregisterHotkey: (hotKeyDefinition: HotKeyDefinition) => void // Unregisters exactly this definition, wherever it sits in the stack
  getRegisteredHotkeys: () => Record<string, HotKeyDefinition[]> // Returns the current registered hotkeys
}

// Maps a caller's definition to the entry stored for it (a copy with the
// normalized combo), so unregisterHotkey removes the caller's own entry
// rather than the top one. Registrations do not always nest: a component
// that re-registers (new callback) or closes while a later one sits above
// it must not take that later hotkey with it.
const storedEntries = new WeakMap<HotKeyDefinition, HotKeyDefinition>()

export const useHotKeysStore = create<HotKeysStore>((set, get) => ({
  registeredHotkeys: {},
  registerHotkey: (hotKeyDefinition: HotKeyDefinition) => {
    const normalizedKeyCombo = normalizeHotkeyCombo(hotKeyDefinition.keyCombo)

    const entry: HotKeyDefinition = {
      ...hotKeyDefinition,
      keyCombo: normalizedKeyCombo,
    }
    storedEntries.set(hotKeyDefinition, entry)

    set((state) => {
      const existingHotkeys = state.registeredHotkeys[normalizedKeyCombo] || []
      return {
        registeredHotkeys: {
          ...state.registeredHotkeys,
          [normalizedKeyCombo]: [...existingHotkeys, entry],
        },
      }
    })
  },
  unregisterHotkey: (hotKeyDefinition: HotKeyDefinition) => {
    const normalizedKeyCombo = normalizeHotkeyCombo(hotKeyDefinition.keyCombo)
    const entry = storedEntries.get(hotKeyDefinition)

    set((state) => {
      const existingHotkeys = state.registeredHotkeys[normalizedKeyCombo] || []
      const index = entry ? existingHotkeys.lastIndexOf(entry) : -1
      if (index === -1) {
        return state // Not registered (any more)
      }
      const updatedHotkeys = existingHotkeys.filter((_, i) => i !== index)
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
