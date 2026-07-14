import { isValidElement } from 'react'
import { Navigate } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import ExternalRedirect from '../auth/ExternalRedirect'
import { LoginForm } from './lazy-routes'
import { createLeafWikiRouter } from './router'

function loginRouteElementType(authDisabled: boolean, loginUrl: string) {
  const router = createLeafWikiRouter(
    false,
    authDisabled,
    false,
    false,
    '',
    loginUrl,
  )
  const loginRoute = router.routes.find((route) => route.path === '/login')
  const element = loginRoute?.element
  if (!isValidElement(element)) {
    throw new Error('expected /login route to render an element')
  }
  return element.type
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
