// Records the current route into useLastWikiLocationStore whenever the app is
// not in Settings mode. Mounted once from AppLayout, which stays mounted across
// view/edit/history/settings navigations, so by the time SettingsLayout reads
// the store it holds the location the user came from.

import { useAppMode } from '@/lib/useAppMode'
import { useLastWikiLocationStore } from '@/stores/lastWikiLocation'
import { useEffect } from 'react'
import { useLocation } from 'react-router'

export function useTrackLastWikiLocation() {
  const location = useLocation()
  const appMode = useAppMode()
  const setLocation = useLastWikiLocationStore((s) => s.setLocation)

  useEffect(() => {
    if (appMode === 'settings') return
    setLocation({
      pathname: location.pathname,
      search: location.search,
      state: location.state,
    })
  }, [appMode, location.pathname, location.search, location.state, setLocation])
}
