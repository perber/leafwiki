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

function applyDesignMode(mode: 'light' | 'dark' | 'system') {
  const root = document.documentElement

  let appliedMode: 'light' | 'dark'
  if (mode === 'system') {
    const prefersDark = window.matchMedia(
      '(prefers-color-scheme: dark)',
    ).matches
    appliedMode = prefersDark ? 'dark' : 'light'
  } else {
    appliedMode = mode
  }

  if (appliedMode === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
}

export const useDesignModeStore = create<DesignModeStore>((set) => ({
  mode:
    (localStorage.getItem('design-mode') as 'light' | 'dark' | 'system') ||
    'system',

  setMode: (mode) => {
    localStorage.setItem('design-mode', mode)
    set({ mode })
    // apply the mode to the document
    applyDesignMode(mode)
  },
}))
