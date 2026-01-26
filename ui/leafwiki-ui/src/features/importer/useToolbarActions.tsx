// Hook to provide toolbar actions for the page viewer

import { useEffect } from 'react'
import { useToolbarStore } from '../toolbar/toolbar'

// Hook to set up toolbar actions based on app mode and read-only status
export function useToolbarActions() {
  const setButtons = useToolbarStore((state) => state.setButtons)

  useEffect(() => {
    setButtons([])
  }, [setButtons])
}
