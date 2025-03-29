import { create } from "zustand"

type SelectedPageStore = {
  selectedPageId: string | null
  setSelectedPageId: (id: string | null) => void
}

export const useSelectedPage = create<SelectedPageStore>()((set) => ({
  selectedPageId: null,
  setSelectedPageId: (id) => set({ selectedPageId: id }),
}))