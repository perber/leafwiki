import { getConfig } from '@/lib/api/config'
import { DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES } from '@/lib/config'
import { create } from 'zustand'

type ConfigStore = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  authDisabled: boolean
  maxAssetUploadSizeBytes: number
  error: string | null
  loading: boolean
  hasLoaded: boolean
  loadConfig: () => Promise<void>
}

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  hideLinkMetadataSection: false,
  authDisabled: false,
  maxAssetUploadSizeBytes: DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES,
  error: null,
  loading: false,
  hasLoaded: false,

  loadConfig: async () => {
    set({ loading: true, error: null })
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
        error: null,
        hasLoaded: true,
      })
    } catch (error) {
      console.warn('Error loading configuration:', error)
      set({
        error:
          error instanceof Error
            ? error.message
            : 'Could not load configuration',
        hasLoaded: true,
      })
    } finally {
      set({ loading: false })
    }
  },
}))
