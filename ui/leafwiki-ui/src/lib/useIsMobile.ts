import { useLayoutEffect, useState } from 'react'

export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === 'undefined') return false
    return window.matchMedia('(max-width: 767px)').matches
  })

  useLayoutEffect(() => {
    if (typeof window === 'undefined') return

    const media = window.matchMedia('(max-width: 767px)')
    const handleChange = () => setIsMobile(media.matches)

    media.addEventListener('change', handleChange)
    return () => media.removeEventListener('change', handleChange)
  }, [])

  return isMobile
}
