import {
  isSectionVisible,
  useSettingsSectionContext,
  type SettingsSection,
} from '@/lib/registries/settingsSectionRegistry'
import { type ReactNode } from 'react'
import { Navigate } from 'react-router'

type Props = {
  section: SettingsSection
  children: ReactNode
}

export default function SettingsSectionGuard({ section, children }: Props) {
  const ctx = useSettingsSectionContext()

  if (!isSectionVisible(section, ctx)) {
    return <Navigate to="/settings" replace />
  }

  // A section with an externalHref (e.g. users, when userManagementUrl is
  // set) is a link out, not an internal route — redirect away rather than
  // rendering its Component, mirroring the old /users external-link case.
  if (section.externalHref?.(ctx)) {
    return <Navigate to="/settings" replace />
  }

  return <>{children}</>
}
