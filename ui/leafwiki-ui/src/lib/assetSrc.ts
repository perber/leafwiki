import { withBasePath } from './routePath'

function isAssetSrc(src: string): boolean {
  return (
    src.startsWith('/assets/') ||
    src.startsWith('assets/') ||
    src.startsWith('/api/')
  )
}

/**
 * Normalizes a markdown-referenced asset `src` (an uploaded page attachment)
 * to a router-correct URL, applying the configured base path. Non-asset
 * sources (external URLs, data URIs) are returned unchanged.
 */
export function normalizeAssetSrc(src: string): string {
  if (src.startsWith('/assets/') || src.startsWith('/api/')) {
    return withBasePath(src)
  }

  if (src.startsWith('assets/')) {
    return withBasePath(`/${src}`)
  }

  return src
}

/**
 * Normalizes an asset `src` and appends a `?v=` cache-busting param, so a
 * re-uploaded asset (same filename, new content) doesn't keep showing a
 * stale, browser-cached copy. A `#fragment` on the source (e.g. a PDF's
 * `#page=N` open parameter) is preserved. Non-asset sources are normalized
 * but left unversioned.
 */
export function versionAssetSrc(src: string): string {
  const normalized = normalizeAssetSrc(src)

  if (!isAssetSrc(src)) {
    return normalized
  }

  try {
    const url = new URL(normalized, location.origin)
    if (!url.searchParams.has('v')) {
      url.searchParams.set('v', Date.now().toString())
    }
    return url.toString()
  } catch {
    return normalized
  }
}
