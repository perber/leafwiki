// useAppMode returns the current application mode.
import { useLocation } from 'react-router-dom'

export type AppMode =
  | 'edit'
  | 'view'
  | 'dialog'
  | 'user-management'
  | 'settings'

// based on the current route it will return the app mode
export function useAppMode(): AppMode {
  const location = useLocation()

  if (location.pathname.startsWith('/e/')) {
    return 'edit'
  }

  if (location.pathname.startsWith('/users')) {
    return 'user-management'
  }

  if (location.pathname.startsWith('/settings')) {
    return 'settings'
  }

  return 'view'
}
