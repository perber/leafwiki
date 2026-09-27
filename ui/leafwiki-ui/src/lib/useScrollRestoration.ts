import { useEffect, useLayoutEffect } from 'react'

const DEFAULT_SCROLL_CONTAINER_ID = 'scroll-container'
const SCROLL_STORAGE_PREFIX = 'scroll:visit:'

export function useScrollRestoration(
  restorationKey: string,
  isLoading: boolean,
  containerId: string = DEFAULT_SCROLL_CONTAINER_ID,
) {
  useLayoutEffect(() => {
    // if hash is present, do not restore scroll position
    const hash = window.location.hash
    if (hash) return

    // Search result navigation (?q=) scrolls to the first match instead.
    const searchQuery = (
      new URLSearchParams(window.location.search).get('q') ?? ''
    ).trim()
    if (searchQuery.length >= 3) return

    if (isLoading) return

    const el = document.getElementById(containerId)
    const stored = sessionStorage.getItem(
      `${SCROLL_STORAGE_PREFIX}${restorationKey}`,
    )

    if (el) {
      requestAnimationFrame(() => {
        el.scrollTo({
          top: stored === null ? 0 : Number.parseInt(stored, 10) || 0,
          behavior: 'auto',
        })
      })
    }
  }, [containerId, isLoading, restorationKey])

  useEffect(() => {
    // if hash is present, do not restore scroll position
    const hash = window.location.hash
    if (hash) return

    const searchQuery = (
      new URLSearchParams(window.location.search).get('q') ?? ''
    ).trim()
    if (searchQuery.length >= 3) return

    return () => {
      const el = document.getElementById(containerId)
      if (el) {
        sessionStorage.setItem(
          `${SCROLL_STORAGE_PREFIX}${restorationKey}`,
          String(el.scrollTop),
        )
      }
    }
  }, [containerId, restorationKey])
}
