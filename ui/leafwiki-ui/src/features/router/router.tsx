import { createBrowserRouter, Navigate, RouteObject } from 'react-router-dom'
import {
  BackupSettings,
  BrandingSettings,
  Importer,
  LoginForm,
  MaintenanceSettings,
  PageEditor,
  PageHistoryPage,
  PageViewer,
  PermalinkRedirect,
  RootRedirect,
  SnapshotSettings,
  UserManagement,
} from './lazy-routes'
import ExternalRedirect from '../auth/ExternalRedirect'
import AuthWrapper from './RouterAuthWrapper'
import ReadOnlyWrapper from './RouterReadOnlyWrapper'

export const createLeafWikiRouter = (
  isReadOnlyViewer: boolean,
  authDisabled: boolean,
  enableRevision: boolean,
  userManagementUrl: string,
  loginUrl: string,
  basename?: string,
) =>
  createBrowserRouter(
    [
      {
        path: '/login',
        element: authDisabled ? (
          <Navigate to="/" replace />
        ) : loginUrl ? (
          <ExternalRedirect to={loginUrl} />
        ) : (
          <LoginForm />
        ),
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
          isReadOnlyViewer || authDisabled || userManagementUrl ? (
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
        path: '/settings/snapshots',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <SnapshotSettings />
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
        path: '/settings/maintenance',
        element: isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <MaintenanceSettings />
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
