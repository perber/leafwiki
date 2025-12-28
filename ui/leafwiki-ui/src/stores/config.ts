import { getConfig } from '@/lib/api/config'
import { create } from 'zustand'

type ConfigStore = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  loading: boolean
  loadConfig: () => Promise<void>
}

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  hideLinkMetadataSection: true,
  loading: false,

  loadConfig: async () => {
    set({ loading: true })
    try {
      const config = await getConfig()
      set({
        publicAccess: config.publicAccess,
        hideLinkMetadataSection: config.hideLinkMetadataSection,
      })
    } catch (error) {
      console.warn('Error loading configuration:', error)
    } finally {
      set({ loading: false })
    }
  },
}))
