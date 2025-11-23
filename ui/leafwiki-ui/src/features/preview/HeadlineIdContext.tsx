import { createContext } from 'react'

export type HeadlineIdContextType = {
  getUniqueId: (baseId: string) => string
}

export const HeadlineIdContext = createContext<HeadlineIdContextType | null>(
  null,
)
