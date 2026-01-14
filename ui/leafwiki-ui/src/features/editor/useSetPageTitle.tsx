import { Page } from '@/lib/api/pages'
import { useBrandingStore } from '@/stores/branding'
import { useEffect } from 'react'

export interface UseSetPageTitleOptions {
  page: Page | null
}

// Hook to set the document title based on the page title
export function useSetPageTitle({ page }: UseSetPageTitleOptions) {
  const siteName = useBrandingStore((s) => s.siteName)

  useEffect(() => {
    document.title = page?.title
      ? `${page?.title} - Edit Page – ${siteName}`
      : `Edit Page – ${siteName}`
  }, [page?.title, siteName])
}
