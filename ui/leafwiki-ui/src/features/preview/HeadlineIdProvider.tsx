import { ReactNode, useRef } from 'react'
import { HeadlineIdContext } from './HeadlineIdContext'

export function HeadlineIdProvider({ children }: { children: ReactNode }) {
  const usedIds = useRef<Map<string, number>>(new Map())

  const getUniqueId = (baseId: string): string => {
    const count = usedIds.current.get(baseId) || 0
    usedIds.current.set(baseId, count + 1)

    if (count === 0) {
      return baseId
    }
    return `${baseId}-${count}`
  }

  return (
    <HeadlineIdContext.Provider value={{ getUniqueId }}>
      {children}
    </HeadlineIdContext.Provider>
  )
}
