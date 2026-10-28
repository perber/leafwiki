import { useBrandingStore } from '@/stores/branding'
import { useEffect } from 'react'
import { documentTitle } from './documentTitle'

export interface UseSetTitleOptions {
  title: string
}

// Hook to set the document title based on the page title
export function useSetTitle({ title }: UseSetTitleOptions) {
  const siteNameFromStore = useBrandingStore((s) => s.siteName)

  useEffect(() => {
    document.title = documentTitle(title, siteNameFromStore)
  }, [title, siteNameFromStore])
}
