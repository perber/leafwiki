import i18next from '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { API_BASE_URL } from '../config'
import { ApiLocalizedError, isApiLocalizedErrorResponse } from './errors'

const t = (key: string) => i18next.t(key, { ns: 'auth' })

export type AuthResponse = {
  accessTokenExpiresAt: number
  message: string
  user: {
    id: string
    username: string
    email: string
    role: 'admin' | 'editor' | 'viewer'
    totpEnabled: boolean
  }
}

// Returned by POST /api/auth/login instead of AuthResponse when the account
// has TOTP enabled: password was correct, but no cookies are set yet. Call
// completeTOTPLogin with loginChallengeToken and a TOTP or recovery code to
// finish logging in.
export type LoginChallenge = {
  requiresTotp: true
  loginChallengeToken: string
}

function isLoginChallenge(data: unknown): data is LoginChallenge {
  return (
    !!data &&
    typeof data === 'object' &&
    (data as { requiresTotp?: unknown }).requiresTotp === true
  )
}

const REFRESH_TIMEOUT_MS = 15000
const REFRESH_THRESHOLD_SECONDS = 60

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function getCsrfTokenFromCookie(): string | null {
  if (typeof document === 'undefined') return null

  const hostMatch =
    document.cookie.match(/(?:^|;\s*)__Host-leafwiki_csrf=([^;]+)/) ??
    document.cookie.match(/(?:^|;\s*)leafwiki_csrf=([^;]+)/)

  if (!hostMatch) return null
  try {
    return decodeURIComponent(hostMatch[1])
  } catch {
    return hostMatch[1]
  }
}

export async function fetchMe(): Promise<AuthResponse['user'] | null> {
  const res = await fetch(`${API_BASE_URL}/api/auth/me`, {
    credentials: 'include',
  })
  if (!res.ok) {
    throw new ApiError(`/api/auth/me returned ${res.status}`, res.status)
  }
  const user = await res.json()
  return user ?? null
}

async function postLoginRequest<T>(path: string, body: object): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  if (!res.ok) {
    let errorBody: unknown = null
    try {
      errorBody = await res.json()
    } catch {
      throw new Error(t('login.errorFallback'))
    }

    if (isApiLocalizedErrorResponse(errorBody)) {
      throw new ApiLocalizedError(errorBody.error)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      typeof (errorBody as { error?: unknown }).error === 'string'
    ) {
      throw new Error((errorBody as { error: string }).error)
    }

    throw new Error(t('login.errorFallback'))
  }

  return (await res.json()) as T
}

function applyAuthResponse(data: AuthResponse) {
  const { setAccessTokenExpiresAt, setUser } = useSessionStore.getState()
  setAccessTokenExpiresAt(data.accessTokenExpiresAt)
  setUser(data.user)
}

export async function login(
  identifier: string,
  password: string,
): Promise<AuthResponse | LoginChallenge> {
  const data = await postLoginRequest<AuthResponse | LoginChallenge>(
    '/api/auth/login',
    { identifier, password },
  )

  if (isLoginChallenge(data)) {
    return data
  }

  applyAuthResponse(data)
  return data
}

// completeTOTPLogin finishes a login handshake started by login() when the
// account has TOTP enabled: code is either a current TOTP code or an unused
// recovery code. Only on success are cookies set.
export async function completeTOTPLogin(
  loginChallengeToken: string,
  code: string,
): Promise<AuthResponse> {
  const data = await postLoginRequest<AuthResponse>('/api/auth/login/totp', {
    loginChallengeToken,
    code,
  })
  applyAuthResponse(data)
  return data
}

// requestPasswordReset always resolves (never throws for an unknown
// identifier) — the backend deliberately returns the same response either
// way, so the UI must not try to distinguish "sent" from "no such user".
export async function requestPasswordReset(identifier: string): Promise<void> {
  await postLoginRequest<{ message: string }>('/api/auth/password/forgot', {
    identifier,
  })
}

export type PasswordResetConfirmResponse = {
  user: AuthResponse['user']
}

// confirmPasswordReset does NOT log the user in — a reset revokes every
// existing session for the account (see the backend's
// ConfirmPasswordResetUseCase), so the frontend sends the user to the login
// page afterward instead of calling applyAuthResponse.
export async function confirmPasswordReset(
  token: string,
  newPassword: string,
): Promise<PasswordResetConfirmResponse> {
  return postLoginRequest<PasswordResetConfirmResponse>(
    '/api/auth/password/reset',
    { token, newPassword },
  )
}

// acceptInvite sets the invited user's real password and, unlike
// confirmPasswordReset, logs them straight in — a freshly invited user has
// no prior session to worry about (see the backend's ConfirmInviteUseCase).
export async function acceptInvite(
  token: string,
  newPassword: string,
): Promise<AuthResponse> {
  const data = await postLoginRequest<AuthResponse>('/api/auth/invite/accept', {
    token,
    newPassword,
  })
  applyAuthResponse(data)
  return data
}

export async function logout() {
  const { authDisabled } = useConfigStore.getState()
  if (authDisabled) return
  const headers = new Headers()
  const csrfToken = getCsrfTokenFromCookie()
  if (csrfToken) headers.set('X-CSRF-Token', csrfToken)

  await fetch(`${API_BASE_URL}/api/auth/logout`, {
    method: 'POST',
    credentials: 'include',
    headers,
  }).catch(() => {})
}

export async function fetchWithAuth(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<unknown> {
  const store = useSessionStore.getState()
  const sessionLogout = store.logout
  const config = useConfigStore.getState()
  const authDisabled = config.authDisabled
  const httpRemoteUserEnabled = config.httpRemoteUserEnabled
  const configLoadSucceeded = config.configLoadSucceeded

  const headers = new Headers(options.headers || {})
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const method = (options.method || 'GET').toUpperCase()
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrfToken = getCsrfTokenFromCookie()
    if (csrfToken) {
      headers.set('X-CSRF-Token', csrfToken)
    }
  }

  let originalBody = options.body
  if (
    options.body &&
    typeof options.body === 'object' &&
    !(options.body instanceof FormData)
  ) {
    originalBody = JSON.stringify(options.body)
  }

  const doFetch = async (): Promise<Response> => {
    return fetch(`${API_BASE_URL}${path}`, {
      ...options,
      credentials: 'include',
      headers,
      body: originalBody,
    })
  }

  if (
    shouldRefreshBeforeRequest(
      store.user,
      store.accessTokenExpiresAt,
      authDisabled,
      httpRemoteUserEnabled,
    )
  ) {
    try {
      await ensureRefresh()
    } catch {
      // A refresh failure only proves the session is really gone once the
      // auth mode is confirmed (configLoadSucceeded) — otherwise this could
      // just as easily be a header-auth deployment whose refresh-token call
      // was always going to 422 (accessTokenExpiresAt is never set in that
      // mode, so the check above fires on every request). Forcing this
      // request to fail on that guess — instead of letting the real request
      // decide, the same way it already does when httpRemoteUserEnabled is
      // *confirmed* true — is what caused GitHub #1407 (spurious
      // CSRF-token-missing logouts, autosave breaking mid-edit).
      if (configLoadSucceeded) {
        await clearSessionState(sessionLogout)
        throw new Error(t('apiErrors.unauthorized'))
      }
    }
  }

  let res = await doFetch()

  if (res.status === 401 && retry && !authDisabled && !httpRemoteUserEnabled) {
    try {
      await ensureRefresh()
      res = await doFetch()
    } catch {
      if (configLoadSucceeded) {
        await clearSessionState(sessionLogout)
        throw new Error(t('apiErrors.unauthorized'))
      }
      // Unconfirmed mode: don't force a logout, but don't fabricate success
      // either — `res` still holds the original 401 from above, so it falls
      // through to the generic !res.ok handling below and surfaces as a
      // normal error instead of a silent retry loop.
    }
  }

  if (!res.ok) {
    const errorText = await res.text()
    let errorBody: unknown = null
    try {
      errorBody = errorText ? JSON.parse(errorText) : null
    } catch {
      throw new ApiError(errorText || t('apiErrors.requestFailed'), res.status)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      (errorBody as { error?: unknown }).error === 'validation_error'
    ) {
      throw errorBody
    }

    if (isApiLocalizedErrorResponse(errorBody)) {
      throw new ApiLocalizedError(errorBody.error)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      typeof (errorBody as { error?: unknown }).error === 'string'
    ) {
      throw new ApiError((errorBody as { error: string }).error, res.status)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      typeof (errorBody as { message?: unknown }).message === 'string'
    ) {
      throw new ApiError((errorBody as { message: string }).message, res.status)
    }

    throw new ApiError(t('apiErrors.requestFailed'), res.status)
  }

  try {
    return await res.json()
  } catch {
    return null
  }
}

declare global {
  var __leafwikiRefreshPromise: Promise<void> | null
}

if (typeof globalThis.__leafwikiRefreshPromise === 'undefined') {
  globalThis.__leafwikiRefreshPromise = null
}

function shouldRefreshBeforeRequest(
  user: AuthResponse['user'] | null,
  accessTokenExpiresAt: number | null,
  authDisabled: boolean,
  httpRemoteUserEnabled: boolean,
): boolean {
  if (authDisabled || httpRemoteUserEnabled || user === null) {
    return false
  }

  if (accessTokenExpiresAt === null) {
    return true
  }

  const nowInSeconds = Math.floor(Date.now() / 1000)
  return accessTokenExpiresAt - nowInSeconds <= REFRESH_THRESHOLD_SECONDS
}

async function clearSessionState(sessionLogout: () => Promise<void>) {
  await sessionLogout()
}

export function ensureRefresh(): Promise<void> {
  const { authDisabled, httpRemoteUserEnabled } = useConfigStore.getState()
  // Session-token refresh only applies to session/JWT auth — skip it
  // whenever that's confirmed false, instead of relying on every caller to
  // have already checked httpRemoteUserEnabled itself. Deliberately NOT
  // gated on configLoadSucceeded: attempting a refresh while the auth mode
  // is still unconfirmed is harmless by itself (worst case one wasted call),
  // and skipping it here entirely would permanently disable refresh for
  // session-auth deployments that hit one bad /api/config fetch. The actual
  // risk — treating an unconfirmed-mode refresh failure as confirmed
  // unauthorized and forcing a logout — is guarded at the reaction site in
  // fetchWithAuth instead, not here.
  if (authDisabled || httpRemoteUserEnabled) {
    return Promise.resolve()
  }

  if (!globalThis.__leafwikiRefreshPromise) {
    globalThis.__leafwikiRefreshPromise = refreshAccessToken().finally(() => {
      globalThis.__leafwikiRefreshPromise = null
    })
  }
  return globalThis.__leafwikiRefreshPromise
}

async function refreshAccessToken() {
  const store = useSessionStore.getState()
  const controller = new AbortController()
  const timeoutId = window.setTimeout(
    () => controller.abort(),
    REFRESH_TIMEOUT_MS,
  )

  try {
    const res = await fetch(`${API_BASE_URL}/api/auth/refresh-token`, {
      method: 'POST',
      credentials: 'include',
      signal: controller.signal,
    })

    if (!res.ok) {
      throw new ApiError(t('apiErrors.refreshFailed'), res.status)
    }

    const data: AuthResponse = await res.json()
    store.setAccessTokenExpiresAt(data.accessTokenExpiresAt)
    store.setUser(data.user)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new Error(t('apiErrors.refreshTimedOut'))
    }

    throw error
  } finally {
    window.clearTimeout(timeoutId)
  }
}
