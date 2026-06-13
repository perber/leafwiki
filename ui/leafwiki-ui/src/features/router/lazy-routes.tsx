import { lazy } from 'react'

export const BackupSettings = lazy(() => import('../backup/BackupSettings'))
export const LoginForm = lazy(() => import('../auth/LoginForm'))
export const BrandingSettings = lazy(() => import('../branding/BrandingSettings'))
export const PageEditor = lazy(() => import('../editor/PageEditor'))
export const Importer = lazy(() => import('../importer/Importer'))
export const PageHistoryPage = lazy(() => import('../page/PageHistoryPage'))
export const PermalinkRedirect = lazy(() => import('../page/PermalinkRedirect'))
export const RootRedirect = lazy(() => import('../page/RootRedirect'))
export const UserManagement = lazy(() => import('../users/UserManagement'))
export const PageViewer = lazy(() => import('../viewer/PageViewer'))
