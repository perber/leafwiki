import { Page } from '@/lib/api/pages'
import { useBrandingStore } from '@/stores/branding'
import { useEffect } from 'react'
import { documentTitle } from './documentTitle'

export interface UseSetPageTitleOptions {
  page: Page | null
  // False while a page may still arrive. Public page shells already carry the
  // page title; resetting it here would flash the site name during loading.
  settled?: boolean
}

// Hook to set the document title based on the page title
export function useSetPageTitle({
  page,
  settled = true,
}: UseSetPageTitleOptions) {
  const siteName = useBrandingStore((s) => s.siteName)

  useEffect(() => {
    if (!page && !settled) return
    document.title = documentTitle(page?.title, siteName)
  }, [page, settled, siteName])
}
