export function buildEditUrl(pathname: string): string {
  if (pathname.startsWith('/e/')) {
    return pathname
  }

  if (pathname.startsWith('/')) {
    pathname = pathname.slice(1)
  }

  return `/e/${pathname}`
}

export function buildViewUrl(pathname: string): string {
  if (pathname.startsWith('/e/')) {
    return pathname.slice(3)
  }

  return pathname
}
