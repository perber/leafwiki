// Dialogs store
// is used to manage the state of dialogs in the application

import { create } from 'zustand'

type DialogsStore = {
  dialogType: string | null
  dialogProps: any | null
  openDialog: (dialogType: string, dialogProps?: any) => void
  closeDialog: () => void
}

export const useDialogsStore = create<DialogsStore>((set) => ({
  dialogType: null,
  dialogProps: null,
  openDialog: (dialogType: string, dialogProps?: any) => {
    set({ dialogType, dialogProps })
  },
  closeDialog: () => {
    set({ dialogType: null, dialogProps: null })
  },
}))
