import { BASE_PATH } from './config'

export function normalizeBasePath(base: string | undefined | null): string {
  if (!base || base === '/') return ''
  base = base.startsWith('/') ? base : `/${base}`
  return base.replace(/\/+$/, '') // trailing slashes entfernen
}

const BASE = normalizeBasePath(BASE_PATH)

function ensureLeadingSlash(pathname: string): string {
  if (!pathname) {
    return '/'
  }
  return pathname.startsWith('/') ? pathname : `/${pathname}`
}

export function withBasePath(p: string): string {
  if (!BASE) return p
  return BASE + (p.startsWith('/') ? p : `/${p}`)
}

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

export function buildEditUrl(pathname: string): string {
  let p = stripBasePath(pathname)
  if (p === null) return `/e${ensureLeadingSlash(pathname)}`
  if (p.startsWith('/e/')) return p
  if (p.startsWith('/')) {
    p = p.slice(1)
  }

  return `/e/${p}`
}

export function buildViewUrl(pathname: string): string {
  const stripped = stripBasePath(pathname)
  if (stripped === null) return pathname
  pathname = stripped

  if (pathname.startsWith('/e/')) return pathname.slice(3)
  return pathname
}
