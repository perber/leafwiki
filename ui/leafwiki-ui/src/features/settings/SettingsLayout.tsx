import { useExitModeButton } from '@/features/toolbar/useExitModeButton'
import { Outlet, useNavigate } from 'react-router'

export default function SettingsLayout() {
  const navigate = useNavigate()

  useExitModeButton({
    id: 'exit-settings',
    labelKey: 'exit',
    ns: 'settings',
    hotkeyId: 'settings.exit',
    onExit: () => navigate('/'),
  })

  return <Outlet />
}
