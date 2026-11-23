import { describe, expect, it } from 'vitest'
import { buildOidcLogoutUrl } from './buildOidcLogoutUrl'

describe('buildOidcLogoutUrl', () => {
  // Regression test for a real production incident: a session created
  // before id_token_hint existed has no hint. Authentik (2026.8.x), once
  // post_logout_redirect_uri is registered on the client, rejects a request
  // carrying post_logout_redirect_uri without id_token_hint outright
  // (invalid_request) instead of just ignoring it — an error page, worse
  // than omitting it. Without a hint, drop the redirect URI too and let the
  // IdP show its own "logged out" page, same as before either existed.
  it('omits post_logout_redirect_uri when there is no id_token_hint', () => {
    const url = buildOidcLogoutUrl('https://idp.example.com/end-session', {
      clientId: 'leafwiki',
      postLogoutRedirectUri: 'https://wiki.example.com/login',
    })

    expect(url).toBe('https://idp.example.com/end-session?client_id=leafwiki')
  })

  it('sends post_logout_redirect_uri when id_token_hint is also present', () => {
    const url = buildOidcLogoutUrl('https://idp.example.com/end-session', {
      clientId: 'leafwiki',
      postLogoutRedirectUri: 'https://wiki.example.com/login',
      idTokenHint: 'raw.id.token',
    })

    expect(url).toBe(
      'https://idp.example.com/end-session?client_id=leafwiki&post_logout_redirect_uri=https%3A%2F%2Fwiki.example.com%2Flogin&id_token_hint=raw.id.token',
    )
  })

  it('returns the base URL unchanged when neither param is given', () => {
    expect(buildOidcLogoutUrl('https://idp.example.com/end-session', {})).toBe(
      'https://idp.example.com/end-session',
    )
  })

  it('adds only client_id when the redirect URI is missing', () => {
    const url = buildOidcLogoutUrl('https://idp.example.com/end-session', {
      clientId: 'leafwiki',
    })

    expect(url).toBe('https://idp.example.com/end-session?client_id=leafwiki')
  })

  it('preserves existing query parameters on the end-session URL', () => {
    const url = buildOidcLogoutUrl(
      'https://idp.example.com/end-session?foo=bar',
      { clientId: 'leafwiki' },
    )

    expect(url).toBe(
      'https://idp.example.com/end-session?foo=bar&client_id=leafwiki',
    )
  })

  it('falls back to the base URL unchanged if it fails to parse', () => {
    expect(buildOidcLogoutUrl('not a url', { clientId: 'leafwiki' })).toBe(
      'not a url',
    )
  })

  it('adds only id_token_hint when it is the only param given', () => {
    const url = buildOidcLogoutUrl('https://idp.example.com/end-session', {
      idTokenHint: 'raw.id.token',
    })

    expect(url).toBe(
      'https://idp.example.com/end-session?id_token_hint=raw.id.token',
    )
  })

  it('omits id_token_hint when empty, same as not given', () => {
    const url = buildOidcLogoutUrl('https://idp.example.com/end-session', {
      clientId: 'leafwiki',
      idTokenHint: '',
    })

    expect(url).toBe('https://idp.example.com/end-session?client_id=leafwiki')
  })
})
