import { create } from 'zustand'

type PublicAccessStore = {
  publicAccess: boolean
  loaded: boolean
  setPublicAccess: (value: boolean) => void
  setLoaded: (loaded: boolean) => void
}

export const usePublicAccessStore = create<PublicAccessStore>((set) => ({
  publicAccess: false,
  loaded: false,
  setPublicAccess: (value) => set({ publicAccess: value }),
  setLoaded: (loaded) => set({ loaded }),
}))
