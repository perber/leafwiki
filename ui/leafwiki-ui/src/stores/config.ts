import { getConfig } from '@/lib/api/config'
import { create } from 'zustand'

type ConfigStore = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  loading: boolean
  hasLoaded: boolean
  loadConfig: () => Promise<void>
}

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  hideLinkMetadataSection: false,
  loading: false,
  hasLoaded: false,

  loadConfig: async () => {
    set({ loading: true })
    try {
      const config = await getConfig()
      set({
        publicAccess: config.publicAccess,
        hideLinkMetadataSection: config.hideLinkMetadataSection,
        hasLoaded: true,
      })
    } catch (error) {
      console.warn('Error loading configuration:', error)
      set({ hasLoaded: true })
    } finally {
      set({ loading: false })
    }
  },
}))
