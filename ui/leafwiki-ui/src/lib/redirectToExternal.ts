// Shared navigation mechanism for all "leave the app for an external URL"
// call sites (login/logout redirects in ExternalRedirect and UserToolbar),
// so redirect behavior only needs to change in one place.
export function redirectToExternal(url: string, returnTo?: string) {
  if (!returnTo) {
    window.location.href = url
    return
  }
  const separator = url.includes('?') ? '&' : '?'
  const absoluteReturnTo = `${window.location.origin}${returnTo}`
  window.location.href = `${url}${separator}redirect_uri=${encodeURIComponent(absoluteReturnTo)}`
}
