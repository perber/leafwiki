import { Page } from '@/lib/api/pages'
import { useBrandingStore } from '@/stores/branding'
import { useEffect } from 'react'
import { documentTitle } from './documentTitle'

export interface UseSetPageTitleOptions {
  page: Page | null
}

// Hook to set the document title based on the page title
export function useSetPageTitle({ page }: UseSetPageTitleOptions) {
  const siteName = useBrandingStore((s) => s.siteName)

  useEffect(() => {
    document.title = documentTitle(page?.title, siteName)
  }, [page?.title, siteName])
}
