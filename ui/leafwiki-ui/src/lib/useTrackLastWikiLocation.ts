// Records the current route into useLastWikiLocationStore whenever the app is
// not on a Settings page. Mounted once from AppLayout, which stays mounted
// across view/edit/history/settings navigations, so by the time SettingsLayout
// reads the store it holds the location the user came from.
//
// The Settings check is deliberately an exact `/settings` / `/settings/` match
// rather than `useAppMode()` — the latter uses `startsWith('/settings')`, which
// also matches an ordinary page whose slug happens to begin with "settings".

import { useLastWikiLocationStore } from '@/stores/lastWikiLocation'
import { stripBasePath } from '@/lib/routePath'
import { useEffect } from 'react'
import { useLocation } from 'react-router'

function isSettingsPath(pathname: string) {
  const path = stripBasePath(pathname) ?? pathname
  return path === '/settings' || path.startsWith('/settings/')
}

export function useTrackLastWikiLocation() {
  const location = useLocation()
  const setLocation = useLastWikiLocationStore((s) => s.setLocation)

  useEffect(() => {
    if (isSettingsPath(location.pathname)) return
    setLocation({
      pathname: location.pathname,
      search: location.search,
      state: location.state,
    })
  }, [location.pathname, location.search, location.state, setLocation])
}
