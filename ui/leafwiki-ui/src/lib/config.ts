function readBasePathFromMeta(): string {
  const raw =
    document
      .querySelector('meta[name="base-path"]')
      ?.getAttribute('content')
      ?.trim() ?? ''

  if (!raw || raw.includes('{{')) {
    return ''
  }

  const withSlash = raw.startsWith('/') ? raw : `/${raw}`
  return withSlash.replace(/\/+$/, '')
}

export const BASE_PATH = readBasePathFromMeta()

export const API_BASE_URL = (BASE_PATH ? `${BASE_PATH}` : '').replace(
  /\/+$/,
  '',
)

export const MAX_UPLOAD_SIZE_MB = 50
export const MAX_UPLOAD_SIZE = MAX_UPLOAD_SIZE_MB * 1024 * 1024
export const IMAGE_EXTENSIONS = [
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'bmp',
  'svg',
]
