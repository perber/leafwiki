import { scrollToSearchQuery } from '@/lib/scrollToSearchQuery'
import { useEffect } from 'react'
import { useLocation, useSearchParams } from 'react-router'

type UseScrollToSearchQueryOptions = {
  content?: string
  isLoading?: boolean
}

export function useScrollToSearchQuery({
  content,
  isLoading,
}: UseScrollToSearchQueryOptions) {
  const { hash } = useLocation()
  const [searchParams] = useSearchParams()
  const query = (searchParams.get('q') ?? '').trim()

  useEffect(() => {
    // Headline hash navigation wins when both are present.
    if (isLoading || !content || hash || query.length < 3) return
    return scrollToSearchQuery(query)
  }, [content, isLoading, hash, query])
}
