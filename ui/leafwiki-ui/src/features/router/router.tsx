import { createBrowserRouter, Navigate, RouteObject } from 'react-router-dom'
import LoginForm from '../auth/LoginForm'
import PageEditor from '../editor/PageEditor'
import PageViewer from '../page/PageViewer'
import RootRedirect from '../page/RootRedirect'
import UserManagement from '../users/UserManagement'
import AuthWrapper from './RouterAuthWrapper'
import ReadOnlyWrapper from './RouterReadOnlyWrapper'

export const createLeafWikiRouter = (isReadOnlyViewer: boolean) =>
  createBrowserRouter([
    {
      path: '/login',
      element: <LoginForm />,
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
      element: isReadOnlyViewer ? (
        <Navigate to="/" />
      ) : (
        <AuthWrapper>
          <UserManagement />
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
