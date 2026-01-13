import { useBrandingStore } from '@/stores/branding'
import { useEffect } from 'react'

export interface UseSetTitleOptions {
  title: string
}

// Hook to set the document title based on the page title
export function useSetTitle({ title }: UseSetTitleOptions) {
  const siteNameFromStore = useBrandingStore((s) => s.siteName)

  useEffect(() => {
    document.title = title
      ? `${title} - ${siteNameFromStore}`
      : siteNameFromStore
  }, [title, siteNameFromStore])
}
