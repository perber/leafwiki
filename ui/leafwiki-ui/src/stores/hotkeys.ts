// Hotkeys store
// is used to manage global hotkeys in the application

import { create } from 'zustand'

export type HotKeyDefinition = {
  keyCombo: string
  enabled: boolean
  mode: string[] // defines in which app modes the hotkey is active
  action: () => void
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
    set((state) => {
      const existingHotkeys =
        state.registeredHotkeys[hotKeyDefinition.keyCombo] || []
      return {
        registeredHotkeys: {
          ...state.registeredHotkeys,
          [hotKeyDefinition.keyCombo]: [...existingHotkeys, hotKeyDefinition],
        },
      }
    })
  },
  unregisterHotkey: (keyCombo: string) => {
    set((state) => {
      const existingHotkeys = state.registeredHotkeys[keyCombo] || []
      if (existingHotkeys.length === 0) {
        return state // No hotkey to unregister
      }
      const updatedHotkeys = existingHotkeys.slice(0, -1) // Remove the last registered hotkey
      const newRegisteredHotkeys = { ...state.registeredHotkeys }
      if (updatedHotkeys.length === 0) {
        delete newRegisteredHotkeys[keyCombo] // Remove the key if no hotkeys left
      } else {
        newRegisteredHotkeys[keyCombo] = updatedHotkeys
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
