import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'

type UseScrollToHeadlineOptions = {
  content?: string
  isLoading?: boolean
}

export function useScrollToHeadline({
  content,
  isLoading,
}: UseScrollToHeadlineOptions) {
  const { hash } = useLocation()
  useEffect(() => {
    if (isLoading || !content || !hash) return
    const cancel = scrollToHeadlineHash(hash)
    return cancel
  }, [content, isLoading, hash])
}
