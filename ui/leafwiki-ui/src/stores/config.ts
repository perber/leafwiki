import { getConfig } from '@/lib/api/config'
import { DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES } from '@/lib/config'
import { create } from 'zustand'

type ConfigStore = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  authDisabled: boolean
  maxAssetUploadSizeBytes: number
  loading: boolean
  hasLoaded: boolean
  loadConfig: () => Promise<void>
}

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  hideLinkMetadataSection: false,
  authDisabled: false,
  maxAssetUploadSizeBytes: DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES,
  loading: false,
  hasLoaded: false,

  loadConfig: async () => {
    set({ loading: true })
    try {
      const config = await getConfig()
      const maxAssetUploadSizeBytes = Number.isFinite(
        config.maxAssetUploadSizeBytes,
      )
        ? config.maxAssetUploadSizeBytes
        : DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES

      set({
        publicAccess: config.publicAccess,
        hideLinkMetadataSection: config.hideLinkMetadataSection,
        authDisabled: config.authDisabled,
        maxAssetUploadSizeBytes,
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
