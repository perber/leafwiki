// useAppMode returns the current application mode.
import { stripBasePath } from '@/lib/routePath'
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
  const pathname = stripBasePath(location.pathname) ?? location.pathname

  if (pathname.startsWith('/e/')) {
    return 'edit'
  }

  if (pathname.startsWith('/users')) {
    return 'user-management'
  }

  if (pathname.startsWith('/settings')) {
    return 'settings'
  }

  return 'view'
}
