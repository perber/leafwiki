import { createBrowserRouter, Navigate, RouteObject } from 'react-router-dom'
import LoginForm from '../auth/LoginForm'
import BrandingSettings from '../branding/BrandingSettings'
import PageEditor from '../editor/PageEditor'
import Importer from '../importer/Importer'
import RootRedirect from '../page/RootRedirect'
import UserManagement from '../users/UserManagement'
import PageViewer from '../viewer/PageViewer'
import AuthWrapper from './RouterAuthWrapper'
import ReadOnlyWrapper from './RouterReadOnlyWrapper'

export const createLeafWikiRouter = (
  isReadOnlyViewer: boolean,
  authDisabled: boolean,
) =>
  createBrowserRouter([
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
  ] satisfies RouteObject[])
