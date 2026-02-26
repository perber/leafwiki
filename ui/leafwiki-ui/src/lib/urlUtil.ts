import { BASE_PATH } from './config'

function stripBasePath(pathname: string): string {
  if (BASE_PATH && pathname.startsWith(BASE_PATH)) {
    pathname = pathname.slice(BASE_PATH.length) || '/'
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
