import { lazy } from 'react'

export const AccountSettings = lazy(
  () => import('../settings/account/AccountSettings'),
)

// The 6 admin-only settings sections share one dynamic-import specifier
// (../settings/adminSections) so the bundler collapses them into a single
// chunk, loaded once when an admin opens any one of them — see
// pleaf/root/AI-Gen-Infos/plans/open/features/
// settings-admin-sections-shared-chunk.md.
export const ApiKeysManagement = lazy(() =>
  import('../settings/adminSections').then((m) => ({
    default: m.ApiKeysManagement,
  })),
)
export const BackupSettings = lazy(() =>
  import('../settings/adminSections').then((m) => ({
    default: m.BackupSettings,
  })),
)
export const BrandingSettings = lazy(() =>
  import('../settings/adminSections').then((m) => ({
    default: m.BrandingSettings,
  })),
)
export const Importer = lazy(() =>
  import('../settings/adminSections').then((m) => ({ default: m.Importer })),
)
export const SnapshotSettings = lazy(() =>
  import('../settings/adminSections').then((m) => ({
    default: m.SnapshotSettings,
  })),
)
export const UserManagement = lazy(() =>
  import('../settings/adminSections').then((m) => ({
    default: m.UserManagement,
  })),
)

export const LoginForm = lazy(() => import('../auth/LoginForm'))
export const ForgotPasswordForm = lazy(
  () => import('../auth/ForgotPasswordForm'),
)
export const ResetPasswordPage = lazy(() => import('../auth/ResetPasswordPage'))
export const AcceptInvitePage = lazy(() => import('../auth/AcceptInvitePage'))
export const PageEditor = lazy(() => import('../editor/PageEditor'))
export const PageHistoryPage = lazy(() => import('../page/PageHistoryPage'))
export const PermalinkRedirect = lazy(() => import('../page/PermalinkRedirect'))
export const RootRedirect = lazy(() => import('../page/RootRedirect'))
export { default as PageViewer } from '../viewer/PageViewer'
