import { afterEach, describe, expect, it } from 'vitest'
import { redirectToExternal } from './redirectToExternal'

describe('redirectToExternal', () => {
  const originalLocation = window.location

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation,
    })
  })

  it('preserves the configured query and hash when adding the return URL', () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...originalLocation, href: '' },
    })

    redirectToExternal(
      'https://idp.example.com/login?tenant=a&redirect_uri=https%3A%2F%2Fold.example.com#section',
      '/protected?tab=x#anchor',
    )

    const redirected = new URL(window.location.href)
    expect(redirected.searchParams.get('tenant')).toBe('a')
    expect(redirected.searchParams.getAll('redirect_uri')).toHaveLength(1)
    expect(redirected.searchParams.get('redirect_uri')).toBe(
      `${originalLocation.origin}/protected?tab=x#anchor`,
    )
    expect(redirected.hash).toBe('#section')
  })

  it('preserves query and hash placement for a legacy relative URL', () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...originalLocation, href: '' },
    })

    redirectToExternal('/login?tenant=a#section', '/protected?tab=x#anchor')

    expect(window.location.href.indexOf('?')).toBeLessThan(
      window.location.href.indexOf('#'),
    )
    const redirected = new URL(window.location.href, originalLocation.origin)
    expect(redirected.origin).toBe(originalLocation.origin)
    expect(redirected.searchParams.get('tenant')).toBe('a')
    expect(redirected.searchParams.get('redirect_uri')).toBe(
      `${originalLocation.origin}/protected?tab=x#anchor`,
    )
    expect(redirected.hash).toBe('#section')
  })
})
