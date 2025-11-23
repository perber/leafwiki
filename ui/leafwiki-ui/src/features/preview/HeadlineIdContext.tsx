import { createContext, useContext, useRef, ReactNode } from 'react'

type HeadlineIdContextType = {
  getUniqueId: (baseId: string) => string
}

const HeadlineIdContext = createContext<HeadlineIdContextType | null>(null)

export function useHeadlineId() {
  const context = useContext(HeadlineIdContext)
  if (!context) {
    throw new Error('useHeadlineId must be used within HeadlineIdProvider')
  }
  return context
}

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
