import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type EditorStore = {
  previewVisible: boolean
  setPreviewVisible: (visible: boolean) => void
  togglePreview: () => void
  lineWrap: boolean
  toggleLineWrap: () => void
  autoSave: boolean
  toggleAutoSave: () => void
}

export const useEditorStore = create<EditorStore>()(
  persist(
    (set, get) => ({
      previewVisible: true,
      setPreviewVisible: (visible) => set({ previewVisible: visible }),
      togglePreview: () => set({ previewVisible: !get().previewVisible }),
      lineWrap: true,
      toggleLineWrap: () => set({ lineWrap: !get().lineWrap }),
      autoSave: true,
      toggleAutoSave: () => set({ autoSave: !get().autoSave }),
    }),
    {
      name: 'leafwiki-editor-settings', // localStorage-Key
      partialize: (state) => ({
        previewVisible: state.previewVisible,
        lineWrap: state.lineWrap,
        autoSave: state.autoSave,
      }),
    },
  ),
)
