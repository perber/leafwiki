// Dialogs store
// is used to manage the state of dialogs in the application

import { create } from 'zustand'

type DialogsStore = {
  dialogType: string | null
  dialogProps: Record<string, unknown> | null
  openDialog: (
    dialogType: string,
    dialogProps?: Record<string, unknown>,
  ) => void
  closeDialog: () => void
}

export const useDialogsStore = create<DialogsStore>((set) => ({
  dialogType: null,
  dialogProps: null,
  openDialog: (dialogType: string, dialogProps?: Record<string, unknown>) => {
    set({ dialogType, dialogProps })
  },
  closeDialog: () => {
    set({ dialogType: null, dialogProps: null })
  },
}))
