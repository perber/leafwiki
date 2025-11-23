import { useContext } from 'react'
import { HeadlineIdContext } from './HeadlineIdContext'

export function useHeadlineId() {
  const context = useContext(HeadlineIdContext)
  if (!context) {
    throw new Error('useHeadlineId must be used within HeadlineIdProvider')
  }
  return context
}
