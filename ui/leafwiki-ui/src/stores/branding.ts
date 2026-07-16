import * as brandingAPI from '@/lib/api/branding'
import i18next from '@/lib/i18n'
import { withBasePath } from '@/lib/routePath'
import { create } from 'zustand'

type BrandingStore = {
  siteName: string
  logoFile: string
  faviconFile: string
  logoVersion: number
  faviconVersion: number
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
  logoVersion: 0,
  faviconVersion: 0,
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
      const assetVersion = Date.now()
      applyFavicon(config.faviconFile, assetVersion)
      set({
        siteName: config.siteName,
        logoFile: config.logoFile,
        logoVersion: assetVersion,
        logoExts: config.brandingConstraints.logoExts,
        maxLogoSize: config.brandingConstraints.maxLogoSize,
        faviconFile: config.faviconFile,
        faviconVersion: assetVersion,
        faviconExts: config.brandingConstraints.faviconExts,
        maxFaviconSize: config.brandingConstraints.maxFaviconSize,
        isLoaded: true,
        isLoading: false,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.brandingSaveFailed'),
        isLoading: false,
      })
    }
  },

  updateBranding: async (config) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await brandingAPI.updateBranding(config)
      applyFavicon(updated.faviconFile, Date.now())
      set({
        siteName: updated.siteName,
        logoFile: updated.logoFile,
        faviconFile: updated.faviconFile,
        isLoading: false,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.brandingSaveFailed'),
        isLoading: false,
      })
      throw err
    }
  },

  uploadLogo: async (file) => {
    set({ isLoading: true, error: null })
    try {
      const result = await brandingAPI.uploadBrandingLogo(file)
      const assetVersion = Date.now()
      set({
        logoFile: result.branding.logoFile,
        logoVersion: assetVersion,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.logoUploadFailed'),
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
      const assetVersion = Date.now()
      applyFavicon(result.branding.faviconFile, assetVersion)
      set({
        faviconFile: result.branding.faviconFile,
        faviconVersion: assetVersion,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.faviconUploadFailed'),
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
      const assetVersion = Date.now()
      set({
        logoFile: '',
        logoVersion: assetVersion,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.logoDeleteFailed'),
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
      const assetVersion = Date.now()
      applyFavicon('', assetVersion)
      set({
        faviconFile: '',
        faviconVersion: assetVersion,
      })
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : i18next.t('branding.toast.faviconDeleteFailed'),
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },
}))

function applyFavicon(faviconFile: string, version: number) {
  const link = getOrCreateFaviconLink()
  const faviconPath = faviconFile
    ? withBasePath(`/branding/${faviconFile}`)
    : withBasePath('/favicon.svg')

  link.href = `${faviconPath}?v=${version}`
}

function getOrCreateFaviconLink(): HTMLLinkElement {
  const existing = document.querySelector(
    "link[rel*='icon']",
  ) as HTMLLinkElement | null

  if (existing) {
    return existing
  }

  const link = document.createElement('link')
  link.rel = 'icon'
  document.head.appendChild(link)
  return link
}
