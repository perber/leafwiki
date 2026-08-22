import { createBrowserRouter, Navigate, RouteObject } from 'react-router'
import {
  AcceptInvitePage,
  ForgotPasswordForm,
  LoginForm,
  PageEditor,
  PageHistoryPage,
  PageViewer,
  PermalinkRedirect,
  ResetPasswordPage,
  RootRedirect,
} from './lazy-routes'
import { settingsSections } from '@/lib/registries/settingsSectionRegistry'
import ExternalRedirect from '../auth/ExternalRedirect'
import SettingsIndexRedirect from '../settings/SettingsIndexRedirect'
import SettingsLayout from '../settings/SettingsLayout'
import SettingsSectionGuard from '../settings/SettingsSectionGuard'
import AuthWrapper from './RouterAuthWrapper'
import ReadOnlyWrapper from './RouterReadOnlyWrapper'

export const createLeafWikiRouter = (
  isReadOnlyViewer: boolean,
  authDisabled: boolean,
  enableRevision: boolean,
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
        // Compat redirect — user management now lives at /settings/users;
        // SettingsSectionGuard there preserves the external userManagementUrl
        // special case and the admin-role gate.
        path: '/users',
        element: <Navigate to="/settings/users" replace />,
      },
      {
        path: '/settings',
        element: isReadOnlyViewer ? (
          <Navigate to="/" replace />
        ) : (
          <AuthWrapper>
            <SettingsLayout />
          </AuthWrapper>
        ),
        children: [
          { index: true, element: <SettingsIndexRedirect /> },
          ...settingsSections.map((section) => ({
            path: section.path,
            element: (
              <SettingsSectionGuard section={section}>
                <section.Component />
              </SettingsSectionGuard>
            ),
          })),
        ],
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
