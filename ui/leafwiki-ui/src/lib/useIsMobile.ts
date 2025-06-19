import { useEffect, useState } from 'react'

export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(false)

  useEffect(() => {
    if (typeof window === 'undefined') {
      return () => {}
    }
    const media = window.matchMedia('(max-width: 767px)')

    const handleChange = () => setIsMobile(media.matches)

    handleChange() // initial
    media.addEventListener('change', handleChange)
    return () => media.removeEventListener('change', handleChange)
  }, [])

  return isMobile
}
