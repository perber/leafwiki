import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

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
    scrollToHeadline(hash)
  }, [content, isLoading, hash])
}

function scrollToHeadline(hash: string) {
  const contentContainer = document.querySelector('main') as HTMLElement | null
  if (!contentContainer) return

  const headlineId = decodeURIComponent(hash.substring(1)) // remove leading #

  const headlineElement = document.getElementById(headlineId)
  if (!headlineElement) return // no such headline, do nothing

  function waitUntilHeightStabilizes(
    element: HTMLElement,
    callback: () => void,
    interval = 250,
    maxTotalTime = 3000,
    stableTime = 500,
  ) {
    let lastHeight = element.scrollHeight
    let stableFor = 0
    let elapsedTime = 0

    const checkHeight = () => {
      const currentHeight = element.scrollHeight
      if (currentHeight === lastHeight) {
        stableFor += interval
        if (stableFor >= stableTime) {
          callback()
          return
        }
      } else {
        lastHeight = currentHeight
        stableFor = 0
      }
      elapsedTime += interval
      if (elapsedTime < maxTotalTime) {
        setTimeout(checkHeight, interval)
      } else {
        console.log('Max wait time reached. Proceeding with scroll.')
        callback()
      }
    }

    setTimeout(checkHeight, interval)
  }

  waitUntilHeightStabilizes(contentContainer!, scroll)

  function scroll() {
    if (!contentContainer || !headlineElement) return

    const contentRect = contentContainer.getBoundingClientRect()
    const headlineRect = headlineElement.getBoundingClientRect()

    const offset =
      headlineRect.top - contentRect.top + contentContainer.scrollTop
    contentContainer.scrollTo({ top: offset, behavior: 'smooth' })
  }
}
