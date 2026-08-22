import { lazy } from 'react'

export const AccountSettings = lazy(
  () => import('../settings/account/AccountSettings'),
)
export const ApiKeysManagement = lazy(
  () => import('../apikeys/ApiKeysManagement'),
)
export const BackupSettings = lazy(() => import('../backup/BackupSettings'))
export const LoginForm = lazy(() => import('../auth/LoginForm'))
export const ForgotPasswordForm = lazy(
  () => import('../auth/ForgotPasswordForm'),
)
export const ResetPasswordPage = lazy(() => import('../auth/ResetPasswordPage'))
export const AcceptInvitePage = lazy(() => import('../auth/AcceptInvitePage'))
export const BrandingSettings = lazy(
  () => import('../branding/BrandingSettings'),
)
export const PageEditor = lazy(() => import('../editor/PageEditor'))
export const Importer = lazy(() => import('../importer/Importer'))
export const MaintenanceSettings = lazy(
  () => import('../maintenance/MaintenanceSettings'),
)
export const PageHistoryPage = lazy(() => import('../page/PageHistoryPage'))
export const PermalinkRedirect = lazy(() => import('../page/PermalinkRedirect'))
export const RootRedirect = lazy(() => import('../page/RootRedirect'))
export const SnapshotSettings = lazy(
  () => import('../snapshot/SnapshotSettings'),
)
export const UserManagement = lazy(() => import('../users/UserManagement'))
export { default as PageViewer } from '../viewer/PageViewer'
