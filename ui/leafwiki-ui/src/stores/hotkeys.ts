// Hotkeys store
// is used to manage global hotkeys in the application

import { create } from 'zustand'

export type HotKeyDefinition = {
  keyCombo: string
  enabled: boolean
  action: () => void
}

type HotKeysStore = {
    registeredHotkeys: Record<string, HotKeyDefinition>
    registerHotkey: (hotKeyDefinition: HotKeyDefinition) => void
    unregisterHotkey: (keyCombo: string) => void
    getRegisteredHotkeys: () => Record<string, HotKeyDefinition>
}

export const useHotKeysStore = create<HotKeysStore>((set, get) => ({
    registeredHotkeys: {},
    registerHotkey: (hotKeyDefinition: HotKeyDefinition) => {   
        set((state) => ({
            registeredHotkeys: {
                ...state.registeredHotkeys,
                [hotKeyDefinition.keyCombo]: hotKeyDefinition,
            },
        }))
    },
    unregisterHotkey: (keyCombo: string) => {
        set((state) => {
            const updatedHotkeys = { ...state.registeredHotkeys }
            delete updatedHotkeys[keyCombo]
            return { registeredHotkeys: updatedHotkeys }
        })
    },
    getRegisteredHotkeys: () => {
        return get().registeredHotkeys
    },
}))
