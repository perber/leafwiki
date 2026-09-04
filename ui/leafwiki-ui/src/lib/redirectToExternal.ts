// Shared navigation mechanism for all "leave the app for an external URL"
// call sites (login/logout redirects in ExternalRedirect and UserMenu),
// so redirect behavior only needs to change in one place.
export function redirectToExternal(url: string, returnTo?: string) {
  if (!returnTo) {
    window.location.href = url
    return
  }
  const absoluteReturnTo = `${window.location.origin}${returnTo}`

  try {
    const redirectUrl = new URL(url)
    redirectUrl.searchParams.set('redirect_uri', absoluteReturnTo)
    window.location.href = redirectUrl.toString()
  } catch {
    const hashIndex = url.indexOf('#')
    const base = hashIndex === -1 ? url : url.slice(0, hashIndex)
    const hash = hashIndex === -1 ? '' : url.slice(hashIndex)
    const separator = base.includes('?') ? '&' : '?'
    window.location.href = `${base}${separator}redirect_uri=${encodeURIComponent(absoluteReturnTo)}${hash}`
  }
}
