import { useEffect, useLayoutEffect } from 'react'

const DEFAULT_SCROLL_CONTAINER_ID = 'scroll-container'

export function useScrollRestoration(
  pathname: string,
  isLoading: boolean,
  containerId: string = DEFAULT_SCROLL_CONTAINER_ID,
) {
  useLayoutEffect(() => {
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
