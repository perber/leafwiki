// Builds the URL UserMenu redirects to for an OIDC logout. The provider's
// end_session_endpoint (discovered once at boot, see core/oidc.Service) is
// static and carries no per-session context by itself: without client_id and
// post_logout_redirect_uri, an IdP serving several clients can't tell which
// one asked, or where it's safe to send the browser back to, so it falls
// back to showing its own "logged out" page instead of returning here.
//
// idTokenHint comes from GET /api/auth/oidc/logout-hint, fetched and awaited
// just before this (see UserMenu) — it's the one piece only the server can
// supply, since the raw ID token is otherwise never handed to frontend JS.
//
// postLogoutRedirectUri is sent ONLY when idTokenHint is also present.
// Confirmed against a real Authentik (2026.8.x) with post_logout_redirect_uri
// registered on the client: a request carrying post_logout_redirect_uri
// without id_token_hint is rejected outright (invalid_request,
// id_token_hint_missing) — an error page, not a graceful fallback. A session
// created before this hint existed (or any other lookup failure) has no
// hint, so for that one logout it's strictly safer to omit
// post_logout_redirect_uri too and let the IdP fall back to its own
// "logged out" page, exactly like before id_token_hint existed. The very
// next login/logout cycle has a hint again, so this only ever affects a
// session's first post-upgrade logout.
export function buildOidcLogoutUrl(
  endSessionUrl: string,
  params: {
    clientId?: string
    postLogoutRedirectUri?: string
    idTokenHint?: string
  },
): string {
  const postLogoutRedirectUri = params.idTokenHint
    ? params.postLogoutRedirectUri
    : undefined

  if (!params.clientId && !postLogoutRedirectUri && !params.idTokenHint) {
    return endSessionUrl
  }

  try {
    const url = new URL(endSessionUrl)
    if (params.clientId) {
      url.searchParams.set('client_id', params.clientId)
    }
    if (postLogoutRedirectUri) {
      url.searchParams.set('post_logout_redirect_uri', postLogoutRedirectUri)
    }
    if (params.idTokenHint) {
      url.searchParams.set('id_token_hint', params.idTokenHint)
    }
    return url.toString()
  } catch {
    return endSessionUrl
  }
}
