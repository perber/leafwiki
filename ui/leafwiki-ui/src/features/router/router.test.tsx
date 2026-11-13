import SettingsSectionGuard from '@/features/settings/SettingsSectionGuard'
import { settingsSections } from '@/lib/registries/settingsSectionRegistry'
import { isValidElement } from 'react'
import { Navigate } from 'react-router'
import { describe, expect, it } from 'vitest'
import ExternalRedirect from '../auth/ExternalRedirect'
import { ForgotPasswordForm, LoginForm } from './lazy-routes'
import { createLeafWikiRouter } from './router'

type LeafWikiRouter = ReturnType<typeof createLeafWikiRouter>

// routeElement walks nested route paths (e.g. ['/settings', 'display']) and
// returns the element the innermost route renders.
function routeElement(router: LeafWikiRouter, paths: string[]) {
  let routes = router.routes
  let element: unknown
  for (const path of paths) {
    const route = routes.find((r) => r.path === path)
    element = route?.element
    routes = route?.children ?? []
  }
  if (!isValidElement(element)) {
    throw new Error(`expected route ${paths.join(' > ')} to render an element`)
  }
  return element
}

function loginRouteElementType(authDisabled: boolean, loginUrl: string) {
  const router = createLeafWikiRouter(
    false,
    authDisabled,
    false,
    loginUrl,
    true,
  )
  return routeElement(router, ['/login']).type
}

describe('createLeafWikiRouter /login route', () => {
  it('navigates home when auth is disabled, even if loginUrl is configured', () => {
    expect(loginRouteElementType(true, 'https://idp.example.com/login')).toBe(
      Navigate,
    )
  })

  it('redirects externally when loginUrl is configured and auth is enabled', () => {
    expect(loginRouteElementType(false, 'https://idp.example.com/login')).toBe(
      ExternalRedirect,
    )
  })

  it('renders the local login form otherwise', () => {
    expect(loginRouteElementType(false, '')).toBe(LoginForm)
  })
})

function forgotPasswordRouteElementType(smtpEnabled: boolean) {
  const router = createLeafWikiRouter(false, false, false, '', smtpEnabled)
  return routeElement(router, ['/forgot-password']).type
}

describe('createLeafWikiRouter /forgot-password route', () => {
  it('redirects to /login when SMTP is not configured', () => {
    expect(forgotPasswordRouteElementType(false)).toBe(Navigate)
  })

  it('renders the forgot-password form when SMTP is configured', () => {
    expect(forgotPasswordRouteElementType(true)).toBe(ForgotPasswordForm)
  })
})

describe('createLeafWikiRouter /settings route', () => {
  it('redirects to / for a read-only viewer', () => {
    const router = createLeafWikiRouter(true, false, false, '', true)
    expect(routeElement(router, ['/settings']).type).toBe(Navigate)
  })

  it.each([
    ['the compat /users route', ['/users'], '/settings/users'],
    [
      'the old /settings/toc-display bookmark',
      ['/settings', 'toc-display'],
      '/settings/display',
    ],
  ])('redirects %s to %s', (_name, paths, target) => {
    const router = createLeafWikiRouter(false, false, false, '', true)
    const element = routeElement(router, paths)
    expect(element.type).toBe(Navigate)
    expect(element.props).toMatchObject({ to: target, replace: true })
  })

  it.each(settingsSections.map((section) => [section.path]))(
    'gates /settings/%s through SettingsSectionGuard — regression for the pre-registry backup/snapshots direct-URL bypass',
    (path) => {
      const router = createLeafWikiRouter(false, false, false, '', true)
      expect(routeElement(router, ['/settings', path]).type).toBe(
        SettingsSectionGuard,
      )
    },
  )
})
