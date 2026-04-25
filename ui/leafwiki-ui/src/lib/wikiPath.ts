import { buildViewUrl } from './routePath'

/**
 * Wiki-domain path helpers.
 *
 * This file is responsible for path semantics inside the wiki itself:
 * normalized page paths, parent-page paths, route-to-lookup conversion, and
 * relative Markdown link resolution.
 *
 * It should not deal with the configured browser base path. That responsibility
 * stays in `routePath.ts`.
 */

/**
 * Normalizes a wiki route/path.
 *
 * Rules:
 * - remove query string and hash fragment
 * - ensure a leading slash
 * - remove trailing slashes except for the root path
 */
export function normalizeWikiRoutePath(path: string): string {
  let normalized = path.split('?')[0].split('#')[0]

  if (!normalized.startsWith('/')) {
    normalized = `/${normalized}`
  }

  if (normalized.length > 1) {
    normalized = normalized.replace(/\/+$/, '')
  }

  return normalized
}

/**
 * Converts a normalized wiki route/path into the tree lookup key format.
 *
 * Example: `/docs/getting-started` -> `docs/getting-started`
 */
export function toWikiLookupPath(path: string): string {
  const normalized = normalizeWikiRoutePath(path)
  return normalized === '/' ? '' : normalized.slice(1)
}

/**
 * Converts any supported route variant to the normalized wiki route/path.
 *
 * Examples:
 * - `/docs` -> `/docs`
 * - `/e/docs` -> `/docs`
 * - `/history/docs` -> `/docs`
 */
export function getWikiTargetRoutePath(pathname: string): string {
  return normalizeWikiRoutePath(buildViewUrl(pathname))
}

/**
 * Resolves a relative Markdown link against the current wiki page path.
 *
 * The result is always an absolute wiki route/path without query or hash.
 */
export function resolveWikiLinkPath(currentPath: string, href: string): string {
  const normalizedCurrentPath = normalizeWikiRoutePath(currentPath)
  const folderBase = normalizedCurrentPath.endsWith('/')
    ? normalizedCurrentPath
    : `${normalizedCurrentPath}/`

  const base = new URL(folderBase, 'https://leafwiki.local')
  const url = new URL(href, base)

  return normalizeWikiRoutePath(url.pathname)
}

/**
 * Returns the parent wiki route/path for a page.
 *
 * Top-level pages resolve to `/`.
 */
export function getParentWikiRoutePath(path: string): string {
  const normalized = normalizeWikiRoutePath(path)

  if (normalized === '/') {
    return '/'
  }

  const segments = normalized.split('/').filter(Boolean)
  if (segments.length <= 1) {
    return '/'
  }

  return `/${segments.slice(0, -1).join('/')}`
}

/**
 * Computes the viewer route to open after deleting a page or section.
 *
 * If the deleted page is currently open, or the current route is nested below
 * it, the redirect goes to the deleted page's parent. Otherwise the current
 * route is kept unchanged.
 */
export function getDeleteRedirectRoutePath(
  currentLocationPath: string,
  deletedPagePath: string,
): string {
  const currentRoutePath = normalizeWikiRoutePath(currentLocationPath)
  const currentViewPath = normalizeWikiRoutePath(buildViewUrl(currentRoutePath))
  const deletedRoutePath = normalizeWikiRoutePath(deletedPagePath)

  const isDeletedRouteActive =
    currentViewPath === deletedRoutePath ||
    currentViewPath.startsWith(`${deletedRoutePath}/`)

  if (isDeletedRouteActive) {
    return getParentWikiRoutePath(deletedRoutePath)
  }

  return currentRoutePath
}
