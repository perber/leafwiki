import { BASE_PATH } from './config'

function ensureLeadingSlash(pathname: string): string {
  if (!pathname) {
    return '/'
  }
  return pathname.startsWith('/') ? pathname : `/${pathname}`
}

function stripBasePath(pathname: string): string {
  // Normalize input first to simplify comparisons
  pathname = ensureLeadingSlash(pathname)
  if (!BASE_PATH) {
    return pathname
  }
  // Only strip when pathname is exactly the base path
  if (pathname === BASE_PATH) {
    return '/'
  }
  // Or when it starts with the base path followed by a path separator
  const baseWithSlash = `${BASE_PATH}/`
  if (pathname.startsWith(baseWithSlash)) {
    const stripped = pathname.slice(BASE_PATH.length)
    return ensureLeadingSlash(stripped)
  }

  return pathname
}

export function buildEditUrl(pathname: string): string {
  pathname = stripBasePath(pathname)

  if (pathname.startsWith('/e/')) {
    return pathname
  }

  if (pathname.startsWith('/')) {
    pathname = pathname.slice(1)
  }

  return `/e/${pathname}`
}

export function buildViewUrl(pathname: string): string {
  pathname = stripBasePath(pathname)

  if (pathname.startsWith('/e/')) {
    return pathname.slice(3)
  }

  return pathname
}
