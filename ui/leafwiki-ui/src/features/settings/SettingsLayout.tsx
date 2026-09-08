import { useExitModeButton } from '@/features/toolbar/useExitModeButton'
import { useLastWikiLocationStore } from '@/stores/lastWikiLocation'
import { useCallback } from 'react'
import { Outlet, useNavigate } from 'react-router'

export default function SettingsLayout() {
  const navigate = useNavigate()
  const lastWikiLocation = useLastWikiLocationStore((s) => s.location)

  const onExit = useCallback(() => {
    if (lastWikiLocation) {
      navigate(
        {
          pathname: lastWikiLocation.pathname,
          search: lastWikiLocation.search,
        },
        { state: lastWikiLocation.state },
      )
      return
    }
    navigate('/')
  }, [navigate, lastWikiLocation])

  useExitModeButton({
    id: 'exit-settings',
    labelKey: 'exit',
    ns: 'settings',
    hotkeyId: 'settings.exit',
    onExit,
  })

  return <Outlet />
}
