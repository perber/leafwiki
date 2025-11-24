import { Page } from '@/lib/api/pages'
import { useEffect } from 'react'

export interface UseSetPageTitleOptions {
  page: Page | null
}

// Hook to set the document title based on the page title
export function useSetPageTitle({ page }: UseSetPageTitleOptions) {
  useEffect(() => {
    document.title = page?.title ? `${page?.title} - LeafWiki` : 'LeafWiki'
  }, [page?.title])
}
