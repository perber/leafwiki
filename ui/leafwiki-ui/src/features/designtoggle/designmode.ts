/**
 * The zustand store and logic for design mode (light/dark).
 * It contains the state and actions to toggle between light and dark modes and also manages persistence.
 * per default if no mode is set, the system preference is used.
 */

import { create } from 'zustand/react'

type DesignModeStore = {
  mode: 'light' | 'dark' | 'system'
  setMode: (mode: 'light' | 'dark' | 'system') => void
}

export const useDesignModeStore = create<DesignModeStore>((set) => ({
  mode:
    (localStorage.getItem('design-mode') as 'light' | 'dark' | 'system') ||
    'system',

  setMode: (mode) => {
    localStorage.setItem('design-mode', mode)
    set({ mode })
  },
}))
