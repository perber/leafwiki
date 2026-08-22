import { createBrowserRouter, Navigate, RouteObject } from 'react-router'
import {
  AcceptInvitePage,
  ApiKeysManagement,
  BackupSettings,
  BrandingSettings,
  ForgotPasswordForm,
  Importer,
  LoginForm,
  MaintenanceSettings,
  PageEditor,
  PageHistoryPage,
  PageViewer,
  PermalinkRedirect,
  ResetPasswordPage,
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
  enableApiKeyManagement: boolean,
  userManagementUrl: string,
  loginUrl: string,
  smtpEnabled: boolean,
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
      // Local-account password-reset/invite pages only make sense with real
      // accounts and SMTP configured — gated the same way as /login (auth
      // disabled or an external login URL both mean "no built-in login form",
      // so these can't be reached meaningfully either), plus smtpEnabled
      // specifically since the backend use cases return ErrEmailDisabled
      // otherwise.
      {
        path: '/forgot-password',
        element:
          authDisabled || loginUrl || !smtpEnabled ? (
            <Navigate to="/login" replace />
          ) : (
            <ForgotPasswordForm />
          ),
      },
      {
        path: '/reset-password',
        element:
          authDisabled || loginUrl || !smtpEnabled ? (
            <Navigate to="/login" replace />
          ) : (
            <ResetPasswordPage />
          ),
      },
      {
        path: '/accept-invite',
        element:
          authDisabled || loginUrl || !smtpEnabled ? (
            <Navigate to="/login" replace />
          ) : (
            <AcceptInvitePage />
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
        path: '/settings/api-keys',
        element: !enableApiKeyManagement ? (
          <Navigate to="/" replace />
        ) : isReadOnlyViewer ? (
          <Navigate to="/" />
        ) : (
          <AuthWrapper>
            <ApiKeysManagement />
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
