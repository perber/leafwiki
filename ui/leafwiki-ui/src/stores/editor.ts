import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type AutoSaveStatus = 'idle' | 'saving' | 'paused'

type EditorStore = {
  previewVisible: boolean
  setPreviewVisible: (visible: boolean) => void
  togglePreview: () => void
  previewStacked: boolean
  togglePreviewLayout: () => void
  lineWrap: boolean
  toggleLineWrap: () => void
  autoSaveStatus: AutoSaveStatus
  setAutoSaveStatus: (status: AutoSaveStatus) => void
}

export const useEditorStore = create<EditorStore>()(
  persist(
    (set, get) => ({
      previewVisible: true,
      setPreviewVisible: (visible) => set({ previewVisible: visible }),
      togglePreview: () => set({ previewVisible: !get().previewVisible }),
      previewStacked: false,
      togglePreviewLayout: () => set({ previewStacked: !get().previewStacked }),
      lineWrap: true,
      toggleLineWrap: () => set({ lineWrap: !get().lineWrap }),
      autoSaveStatus: 'idle',
      setAutoSaveStatus: (status) => set({ autoSaveStatus: status }),
    }),
    {
      name: 'leafwiki-editor-settings',
      partialize: (state) => ({
        previewVisible: state.previewVisible,
        previewStacked: state.previewStacked,
        lineWrap: state.lineWrap,
      }),
    },
  ),
)
