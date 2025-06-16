import { useSidebarStore } from '@/stores/sidebar'
import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

export function useAutoCloseSidebarOnMobile() {
  const location = useLocation()
  const setSidebarVisible = useSidebarStore((s) => s.setSidebarVisible)

  useEffect(() => {
    if (window.innerWidth < 768) {
      setSidebarVisible(false)
    }
  }, [location.pathname, setSidebarVisible])
}
