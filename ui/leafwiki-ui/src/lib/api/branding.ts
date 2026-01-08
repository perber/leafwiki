import { API_BASE_URL } from '../config'
import { fetchWithAuth } from './auth'

export type BrandingConfig = {
    siteName: string
    logoImagePath: string
    faviconImagePath: string
}

export async function getBranding(): Promise<BrandingConfig> {
    const res = await fetch(`${API_BASE_URL}/api/branding`)
    if (!res.ok) {
        throw new Error(`Failed to load branding: ${res.status} ${res.statusText}`)
    }
    return await res.json()
}

export async function updateBranding(
    config: Partial<BrandingConfig>,
): Promise<BrandingConfig> {
    return (await fetchWithAuth('/api/branding', {
        method: 'PUT',
        body: JSON.stringify(config),
    })) as BrandingConfig
}

export async function uploadBrandingLogo(
    file: File,
): Promise<{ path: string; branding: BrandingConfig }> {
    const formData = new FormData()
    formData.append('file', file)

    return (await fetchWithAuth('/api/branding/logo', {
        method: 'POST',
        body: formData,
        headers: {}, // Let browser set Content-Type for FormData
    })) as { path: string; branding: BrandingConfig }
}

export async function uploadBrandingFavicon(
    file: File,
): Promise<{ path: string; branding: BrandingConfig }> {
    const formData = new FormData()
    formData.append('file', file)

    return (await fetchWithAuth('/api/branding/favicon', {
        method: 'POST',
        body: formData,
        headers: {}, // Let browser set Content-Type for FormData
    })) as { path: string; branding: BrandingConfig }
}
