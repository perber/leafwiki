// useAppMode returns the current application mode.
import { useLocation } from 'react-router-dom'

export type AppMode = 'edit' | 'view'

// based on the current route it will return the app mode
export function useAppMode(): AppMode {
  const location = useLocation()

  if (location.pathname.startsWith('/e/')) {
    return 'edit'
  }

  return 'view'
}
