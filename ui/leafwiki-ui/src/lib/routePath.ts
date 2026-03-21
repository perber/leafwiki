import { BASE_PATH } from './config'

/**
 * Browser/router URL helpers.
 *
 * This file is responsible for route URLs that may include the configured
 * frontend base path and the viewer/editor route prefixes.
 *
 * It should not contain wiki-domain rules such as parent-page calculation or
 * Markdown link resolution. Those belong in `wikiPath.ts`.
 */
export function normalizeBasePath(base: string | undefined | null): string {
  if (!base || base === '/') return ''
  base = base.startsWith('/') ? base : `/${base}`
  return base.replace(/\/+$/, '')
}

const BASE = normalizeBasePath(BASE_PATH)

/** Ensures a pathname is represented as a route path with a leading slash. */
function ensureLeadingSlash(pathname: string): string {
  if (!pathname) {
    return '/'
  }
  return pathname.startsWith('/') ? pathname : `/${pathname}`
}

/** Adds the configured router base path to an already computed route path. */
export function withBasePath(p: string): string {
  if (!BASE) return p
  return BASE + (p.startsWith('/') ? p : `/${p}`)
}

/**
 * Removes the configured router base path from a browser pathname.
 *
 * Returns `null` when the pathname does not belong to the configured base.
 */
export function stripBasePath(pathname: string): string | null {
  pathname = ensureLeadingSlash(pathname)
  if (!BASE) return pathname
  if (pathname === BASE) return '/'
  if (pathname.startsWith(BASE + '/')) {
    const stripped = pathname.slice(BASE.length)
    return ensureLeadingSlash(stripped)
  }
  return null
}

/**
 * Builds the internal editor route for a wiki page path.
 *
 * Input may already be a browser pathname or a wiki path. The result is always
 * a router path without the configured browser base path.
 */
export function buildEditUrl(pathname: string): string {
  let p = stripBasePath(pathname)
  if (p === null) return `/e${ensureLeadingSlash(pathname)}`
  if (p.startsWith('/e/')) return p
  if (p.startsWith('/')) {
    p = p.slice(1)
  }

  return `/e/${p}`
}

/**
 * Builds the browser URL for the editor, including the configured base path.
 */
export function buildBrowserEditUrl(pathname: string): string {
  const normalized = ensureLeadingSlash(pathname)
  if (normalized.startsWith('/e/')) {
    return withBasePath(normalized)
  }
  return withBasePath(buildEditUrl(pathname))
}

/**
 * Converts a router pathname back to the viewer route.
 *
 * This removes the configured base path and strips the `/e/` editor prefix when
 * present. The result is still a router/view route path, not a lookup key.
 */
export function buildViewUrl(pathname: string): string {
  const stripped = stripBasePath(pathname)
  if (stripped === null) return pathname
  pathname = stripped

  if (pathname.startsWith('/e/')) return pathname.slice(3)
  return pathname
}
