import {
  isSectionVisible,
  settingsSections,
  useSettingsSectionContext,
} from '@/lib/registries/settingsSectionRegistry'
import { Navigate } from 'react-router'

export default function SettingsIndexRedirect() {
  const ctx = useSettingsSectionContext()

  // Only ever redirect to an internally-rendered, visible section — a
  // section with an externalHref set is a link out, not a route, so
  // picking it here would bounce straight back via SettingsSectionGuard.
  const target = settingsSections.find(
    (section) => isSectionVisible(section, ctx) && !section.externalHref?.(ctx),
  )

  return <Navigate to={target ? `/settings/${target.path}` : '/'} replace />
}
