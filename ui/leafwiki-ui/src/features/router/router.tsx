import { lazy } from 'react'
import { createBrowserRouter, Navigate, RouteObject } from 'react-router-dom'
import AuthWrapper from './RouterAuthWrapper'
import ReadOnlyWrapper from './RouterReadOnlyWrapper'

const BackupSettings = lazy(() => import('../backup/BackupSettings'))
const LoginForm = lazy(() => import('../auth/LoginForm'))
const BrandingSettings = lazy(() => import('../branding/BrandingSettings'))
const PageEditor = lazy(() => import('../editor/PageEditor'))
const Importer = lazy(() => import('../importer/Importer'))
const PageHistoryPage = lazy(() => import('../page/PageHistoryPage'))
const PermalinkRedirect = lazy(() => import('../page/PermalinkRedirect'))
const RootRedirect = lazy(() => import('../page/RootRedirect'))
const UserManagement = lazy(() => import('../users/UserManagement'))
const PageViewer = lazy(() => import('../viewer/PageViewer'))

export const createLeafWikiRouter = (
  isReadOnlyViewer: boolean,
  authDisabled: boolean,
  enableRevision: boolean,
  basename?: string,
) =>
  createBrowserRouter(
    [
      {
        path: '/login',
        element: authDisabled ? <Navigate to="/" replace /> : <LoginForm />,
      },
      {
        path: '/',
        element: isReadOnlyViewer ? (
          <ReadOnlyWrapper>
            <RootRedirect />
          </ReadOnlyWrapper>
        ) : (
          <AuthWrapper>
            <RootRedirect />
          </AuthWrapper>
        ),
      },
      {
        path: '/users',
        element:
          isReadOnlyViewer || authDisabled ? (
            <Navigate to="/" />
          ) : (
            <AuthWrapper>
              <UserManagement />
            </AuthWrapper>
          ),
      },
      {
        path: '/settings/branding',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <BrandingSettings />
          </AuthWrapper>
        ),
      },
      {
        path: '/settings/backup',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <BackupSettings />
          </AuthWrapper>
        ),
      },
      {
        path: '/settings/importer',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <Importer />
          </AuthWrapper>
        ),
      },
      {
        path: '/settings',
        element: isReadOnlyViewer ? (
          <Navigate to="/" replace />
        ) : (
          <Navigate to="/settings/branding" replace />
        ),
      },
      {
        path: '/e/*',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <PageEditor />
          </AuthWrapper>
        ),
      },
      {
        path: '/history/*',
        element: !enableRevision ? (
          <Navigate to="/" replace />
        ) : isReadOnlyViewer ? (
          <ReadOnlyWrapper>
            <PageHistoryPage />
          </ReadOnlyWrapper>
        ) : (
          <AuthWrapper>
            <PageHistoryPage />
          </AuthWrapper>
        ),
      },
      {
        path: '/p/:id/:slug?',
        element: isReadOnlyViewer ? (
          <ReadOnlyWrapper>
            <PermalinkRedirect />
          </ReadOnlyWrapper>
        ) : (
          <AuthWrapper>
            <PermalinkRedirect />
          </AuthWrapper>
        ),
      },
      {
        path: '*',
        element: isReadOnlyViewer ? (
          <ReadOnlyWrapper>
            <PageViewer />
          </ReadOnlyWrapper>
        ) : (
          <AuthWrapper>
            <PageViewer />
          </AuthWrapper>
        ),
      },
    ] satisfies RouteObject[],
    { basename: basename || undefined },
  )
