export const BASE_PATH = (
  document.querySelector('meta[name="base-path"]')?.getAttribute('content') ||
  ''
).replace(/\/+$/, '')

export const API_BASE_URL = (import.meta.env.VITE_API_URL || BASE_PATH).replace(
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
