import { useEffect } from 'react'
import { useToolbarStore } from '../toolbar/toolbarStore'

export function useToolbarActions() {
  const setButtons = useToolbarStore((state) => state.setButtons)

  useEffect(() => {
    setButtons([])
  }, [setButtons])
}
