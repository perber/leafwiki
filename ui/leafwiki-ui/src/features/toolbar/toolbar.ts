// toolbar.ts
// zustand store for toolbar state management
import { create } from 'zustand'

export interface ToolbarButton {
  id: string
  label: string
  tooltip?: string
  hotkey: string
  icon: React.ReactNode
  variant?: 'outline' | 'ghost' | 'link' | 'destructive' | 'default'
  className?: string
  destructive?: boolean
  disabled?: boolean
  action: () => void
}

export interface ToolbarState {
  buttons: ToolbarButton[]
  setButtons: (buttons: ToolbarButton[]) => void
  getButtons: () => ToolbarButton[]
}

export const useToolbarStore = create<ToolbarState>((set, get) => ({
  buttons: [],
  setButtons: (buttons) => set({ buttons }),
  getButtons: () => get().buttons,
}))
