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
  isAnyDialogOpen: () => boolean
  closeDialog: () => void
}

export const useDialogsStore = create<DialogsStore>((set, get) => ({
  dialogType: null,
  dialogProps: null,
  openDialog: (dialogType: string, dialogProps?: Record<string, unknown>) => {
    set({ dialogType, dialogProps })
  },
  isAnyDialogOpen: () => {
    return get().dialogType !== null
  },
  closeDialog: () => {
    set({ dialogType: null, dialogProps: null })
  },
}))
