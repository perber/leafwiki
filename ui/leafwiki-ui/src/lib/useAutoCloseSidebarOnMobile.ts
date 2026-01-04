import { useIsMobile } from '@/lib/useIsMobile'
import { useSidebarStore } from '@/stores/sidebar'
import { useLayoutEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'

export function useAutoCloseSidebarOnMobile() {
  const location = useLocation()
  const isMobile = useIsMobile()

  const sidebarVisible = useSidebarStore((s) => s.sidebarVisible)
  const setSidebarVisible = useSidebarStore((s) => s.setSidebarVisible)

  const prevPathRef = useRef(location.pathname)

  useLayoutEffect(() => {
    if (!isMobile) {
      prevPathRef.current = location.pathname
      return
    }

    const prevPath = prevPathRef.current
    const nextPath = location.pathname
    prevPathRef.current = nextPath

    // close sidebar on mobile when path changes
    if (prevPath !== nextPath && sidebarVisible) {
      setSidebarVisible(false)
    }
  }, [location.pathname, isMobile, sidebarVisible, setSidebarVisible])
}
