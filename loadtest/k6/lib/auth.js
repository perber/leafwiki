// Shared helpers for the LeafWiki load-test k6 scripts.
//
// Session pool entries come from loadtest/seed/create-users.sh, shaped as
// { username, cookie, csrfToken }. `cookie` is a pre-built
// "leafwiki_at=...; leafwiki_rt=...; leafwiki_csrf=..." header value — using
// the Cookie header directly (rather than k6's cookie jar) keeps each VU's
// session fully independent and avoids jar/domain edge cases.

export function pickSession(pool, vu) {
  return pool[(vu - 1) % pool.length];
}

export function authHeaders(session, extra) {
  const headers = {
    'Content-Type': 'application/json',
    Cookie: session.cookie,
  };
  if (extra) {
    Object.assign(headers, extra);
  }
  return headers;
}

export function writeHeaders(session) {
  return authHeaders(session, { 'X-CSRF-Token': session.csrfToken });
}
