import { useEffect } from 'react'

/**
 * Restores and saves scroll position for a custom scroll container.
 *
 * @param pathname - current route path
 * @param isLoading - whether the page is still loading (delays restoration)
 * @param containerId - the DOM id of the scroll container (default: 'scroll-container')
 */
export function useScrollRestoration(
  pathname: string,
  isLoading: boolean,
  containerId: string = 'scroll-container',
) {
  useEffect(() => {
    if (isLoading) return

    const el = document.getElementById(containerId)
    const stored = sessionStorage.getItem(`scroll:${pathname}`)

    if (el && stored !== null) {
      requestAnimationFrame(() => {
        el.scrollTo({ top: parseInt(stored, 10), behavior: 'auto' })
      })
    }
  }, [isLoading, pathname, containerId])

  useEffect(() => {
    return () => {
      const el = document.getElementById(containerId)
      if (el) {
        sessionStorage.setItem(`scroll:${pathname}`, String(el.scrollTop))
      }
    }
  }, [pathname, containerId])
}
