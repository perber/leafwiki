import * as brandingAPI from '@/lib/api/branding'
import { create } from 'zustand'

type BrandingStore = {
  siteName: string
  logoImagePath: string
  faviconImagePath: string
  isLoaded: boolean
  isLoading: boolean
  error: string | null

  loadBranding: () => Promise<void>
  updateBranding: (config: Partial<brandingAPI.BrandingConfig>) => Promise<void>
  uploadLogo: (file: File) => Promise<void>
  uploadFavicon: (file: File) => Promise<void>
}

export const useBrandingStore = create<BrandingStore>((set) => ({
  siteName: 'LeafWiki',
  logoImagePath: '',
  faviconImagePath: '',
  isLoaded: false,
  isLoading: false,
  error: null,

  loadBranding: async () => {
    set({ isLoading: true, error: null })
    try {
      const config = await brandingAPI.getBranding()
      set({
        siteName: config.siteName,
        logoImagePath: config.logoImagePath,
        faviconImagePath: config.faviconImagePath,
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
        logoImagePath: updated.logoImagePath,
        faviconImagePath: updated.faviconImagePath,
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
        logoImagePath: result.branding.logoImagePath,
        isLoading: false,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to upload logo',
        isLoading: false,
      })
      throw err
    }
  },

  uploadFavicon: async (file) => {
    set({ isLoading: true, error: null })
    try {
      const result = await brandingAPI.uploadBrandingFavicon(file)
      set({
        faviconImagePath: result.branding.faviconImagePath,
        isLoading: false,
      })
      // Refresh favicon in browser
      refreshFavicon()
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to upload favicon',
        isLoading: false,
      })
      throw err
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
