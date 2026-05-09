type ScrollToHeadlineOptions = {
  behavior?: ScrollBehavior
  waitForStableLayout?: boolean
}

export function scrollToHeadlineHash(
  hash: string,
  {
    behavior = 'smooth',
    waitForStableLayout = true,
  }: ScrollToHeadlineOptions = {},
) {
  const contentContainer = document.getElementById(
    'scroll-container',
  ) as HTMLElement | null
  if (!contentContainer) return

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
        callback()
      }
    }

    setTimeout(checkHeight, interval)
  }

  const scrollToTarget = () => {
    const rawHeadlineId = hash.substring(1)
    let headlineId = rawHeadlineId

    try {
      headlineId = decodeURIComponent(rawHeadlineId)
    } catch {
      headlineId = rawHeadlineId
    }

    const headlineElement = document.getElementById(headlineId)
    if (!headlineElement) {
      console.warn(`Headline with id "${headlineId}" not found.`)
      return
    }
    headlineElement.scrollIntoView({ behavior, block: 'start' })
  }

  if (waitForStableLayout) {
    waitUntilHeightStabilizes(contentContainer, scrollToTarget)
    return
  }

  scrollToTarget()
}
