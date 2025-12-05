// useAppMode returns the current application mode.
import { useLocation } from 'react-router-dom'

export type AppMode = 'edit' | 'view' | 'dialog' | 'user-management'

// based on the current route it will return the app mode
export function useAppMode(): AppMode {
  const location = useLocation()

  if (location.pathname.startsWith('/e/')) {
    return 'edit'
  }

  if (location.pathname.startsWith('/users')) {
    return 'user-management'
  }

  return 'view'
}
