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

export const DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES = 50 * 1024 * 1024

export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }

  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes
  let unitIndex = -1

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }

  const fractionDigits = value >= 10 ? 0 : 1
  return `${value.toFixed(fractionDigits)} ${units[unitIndex]}`
}

export const IMAGE_EXTENSIONS = [
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'bmp',
  'svg',
]
