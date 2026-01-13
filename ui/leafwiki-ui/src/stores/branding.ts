import * as brandingAPI from '@/lib/api/branding'
import { create } from 'zustand'

type BrandingStore = {
  siteName: string
  logoFile: string
  faviconFile: string
  logoExts: string[]
  maxLogoSize: number
  faviconExts: string[]
  maxFaviconSize: number
  isLoaded: boolean
  isLoading: boolean
  error: string | null

  loadBranding: () => Promise<void>
  updateBranding: (config: Partial<brandingAPI.BrandingConfig>) => Promise<void>
  uploadLogo: (file: File) => Promise<void>
  uploadFavicon: (file: File) => Promise<void>
  deleteLogo: () => Promise<void>
  deleteFavicon: () => Promise<void>
}

export const useBrandingStore = create<BrandingStore>((set) => ({
  siteName: 'LeafWiki',
  logoFile: '',
  faviconFile: '',
  logoExts: [],
  maxLogoSize: 0,
  faviconExts: [],
  maxFaviconSize: 0,
  isLoaded: false,
  isLoading: false,
  error: null,

  loadBranding: async () => {
    set({ isLoading: true, error: null })
    try {
      const config = await brandingAPI.getBranding()
      set({
        siteName: config.siteName,
        logoFile: config.logoFile,
        logoExts: config.brandingConstraints.logoExts,
        maxLogoSize: config.brandingConstraints.maxLogoSize,
        faviconFile: config.faviconFile,
        faviconExts: config.brandingConstraints.faviconExts,
        maxFaviconSize: config.brandingConstraints.maxFaviconSize,
        isLoaded: true,
        isLoading: false,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to load branding',
        isLoading: false,
      })
    }
  },

  updateBranding: async (config) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await brandingAPI.updateBranding(config)
      set({
        siteName: updated.siteName,
        logoFile: updated.logoFile,
        faviconFile: updated.faviconFile,
        isLoading: false,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to update branding',
        isLoading: false,
      })
      throw err
    }
  },

  uploadLogo: async (file) => {
    set({ isLoading: true, error: null })
    try {
      const result = await brandingAPI.uploadBrandingLogo(file)
      set({
        logoFile: result.branding.logoFile,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to upload logo',
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },

  uploadFavicon: async (file) => {
    set({ isLoading: true, error: null })
    try {
      const result = await brandingAPI.uploadBrandingFavicon(file)
      set({
        faviconFile: result.branding.faviconFile,
      })
      // Refresh favicon in browser
      refreshFavicon()
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to upload favicon',
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },

  deleteLogo: async () => {
    set({ isLoading: true, error: null })
    try {
      await brandingAPI.deleteBrandingLogo()
      set({
        logoFile: '',
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to delete logo',
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },

  deleteFavicon: async () => {
    set({ isLoading: true, error: null })
    try {
      await brandingAPI.deleteBrandingFavicon()
      set({
        faviconFile: '',
      })
      // Refresh favicon in browser
      refreshFavicon()
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to delete favicon',
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },
}))

// Helper to refresh favicon in browser
function refreshFavicon() {
  const link = document.querySelector(
    "link[rel*='icon']",
  ) as HTMLLinkElement | null
  if (link) {
    const href = link.href.split('?')[0]
    link.href = `${href}?v=${Date.now()}`
  }
}
