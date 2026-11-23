import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { completeTOTPLogin, login } from '@/lib/api/auth'
import { mapApiError } from '@/lib/api/errors'
import { API_BASE_URL } from '@/lib/config'
import { redirectToExternal } from '@/lib/redirectToExternal'
import { withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Link,
  Navigate,
  useLocation,
  useNavigate,
  useSearchParams,
} from 'react-router'
import { toast } from 'sonner'

function getRedirectTo(state: unknown): string | null {
  if (!state || typeof state !== 'object') {
    return null
  }

  const redirectTo = (state as { redirectTo?: unknown }).redirectTo
  if (typeof redirectTo !== 'string' || !redirectTo.startsWith('/')) {
    return null
  }

  if (
    redirectTo === '/login' ||
    redirectTo.startsWith('/login?') ||
    redirectTo.startsWith('/login#')
  ) {
    return null
  }

  return redirectTo
}

export default function LoginForm() {
  const { t } = useTranslation('auth')
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [loginChallengeToken, setLoginChallengeToken] = useState<string | null>(
    null,
  )
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)

  const location = useLocation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const user = useSessionStore((s) => s.user)
  const smtpEnabled = useConfigStore((s) => s.smtpEnabled)
  const oidcEnabled = useConfigStore((s) => s.oidcEnabled)
  const oidcButtonLabel = useConfigStore((s) => s.oidcButtonLabel)
  const oidcOnly = useConfigStore((s) => s.oidcOnly)
  const { siteName, logoFile, logoVersion } = useBrandingStore()
  const redirectTo = getRedirectTo(location.state)
  // Undocumented in the UI on purpose — an admin locked out by OIDC_ONLY
  // falls back to this via the README, not a visible link.
  const localEscapeHatch = searchParams.get('local') === '1'
  const showLocalForm = !oidcOnly || localEscapeHatch

  useEffect(() => {
    if (searchParams.get('oidc_error')) {
      toast.error(t('login.oidcErrorToast'))
    }
    // Only ever show this once, right after the OIDC callback redirect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // If already logged in, redirect to home
  if (user) {
    return <Navigate to={redirectTo || '/'} replace />
  }

  const handleOidcLogin = () => {
    redirectToExternal(`${API_BASE_URL}/api/auth/oidc/login`)
  }

  const handleCredentialsSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      const result = await login(identifier, password)
      if ('requiresTotp' in result) {
        setLoginChallengeToken(result.loginChallengeToken)
        return
      }
      // user already set in the store by the login function
      // Restore the originally requested page after successful login.
      navigate(redirectTo || '/', { replace: true })
    } catch (err) {
      const mapped = mapApiError(err, t('login.errorFallback'))
      toast.error(mapped.message)
    } finally {
      setLoading(false)
    }
  }

  const handleTotpSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!loginChallengeToken) return
    setLoading(true)

    try {
      // user already set in the store by completeTOTPLogin
      await completeTOTPLogin(loginChallengeToken, code)
      navigate(redirectTo || '/', { replace: true })
    } catch (err) {
      const mapped = mapApiError(err, t('login.totp.errorFallback'))
      toast.error(mapped.message)
      setCode('')
    } finally {
      setLoading(false)
    }
  }

  const logoHeader = (
    <h1 className="login__title">
      {logoFile ? (
        <img
          src={`${withBasePath(`/branding/${logoFile}`)}?v=${logoVersion}`}
          alt={siteName}
          className="login__logo-image"
        />
      ) : (
        <span>🌿</span>
      )}{' '}
      {siteName}
    </h1>
  )

  if (loginChallengeToken) {
    return (
      <>
        <title>{t('login.pageTitle', { siteName })}</title>
        <div className="login">
          <form onSubmit={handleTotpSubmit} className="login__form">
            {logoHeader}
            <p className="login__totp-description">
              {t('login.totp.description')}
            </p>

            <div className="login__field">
              <Input
                type="text"
                placeholder={t('login.totp.codePlaceholder')}
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
                name="code"
                autoComplete="one-time-code"
                autoFocus
                data-testid="login-totp-code"
                spellCheck={false}
              />
            </div>

            <Button
              type="submit"
              className="login__submit"
              disabled={loading}
              data-testid="login-totp-submit"
            >
              {loading ? t('login.totp.submitting') : t('login.totp.submit')}
            </Button>
            <Button
              type="button"
              variant="ghost"
              className="login__totp-back"
              disabled={loading}
              onClick={() => {
                setLoginChallengeToken(null)
                setCode('')
              }}
            >
              {t('login.totp.back')}
            </Button>
          </form>
        </div>
      </>
    )
  }

  return (
    <>
      <title>{t('login.pageTitle', { siteName })}</title>
      <div className="login">
        <div className="login__form">
          {logoHeader}

          {oidcEnabled && (
            <Button
              type="button"
              variant="outline"
              className="login__submit mb-4"
              onClick={handleOidcLogin}
              data-testid="login-oidc-button"
            >
              {t('login.oidcButton', { label: oidcButtonLabel })}
            </Button>
          )}

          {showLocalForm && (
            <form onSubmit={handleCredentialsSubmit}>
              <div className="login__field">
                <Input
                  type="text"
                  placeholder={t('login.identifierPlaceholder')}
                  value={identifier}
                  onChange={(e) => setIdentifier(e.target.value)}
                  required
                  name="identifier"
                  autoComplete="username"
                  data-testid="login-identifier"
                  spellCheck={false}
                />
              </div>

              <div className="login__field">
                <Input
                  type="password"
                  placeholder={t('login.passwordPlaceholder')}
                  name="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  data-testid="login-password"
                  autoComplete="current-password"
                  spellCheck={false}
                />
              </div>

              {smtpEnabled && (
                <Link
                  to="/forgot-password"
                  className="login__forgot-password"
                  data-testid="login-forgot-password-link"
                >
                  {t('login.forgotPasswordLink')}
                </Link>
              )}

              <Button
                type="submit"
                className="login__submit"
                disabled={loading}
                data-testid="login-submit"
              >
                {loading ? t('login.submitting') : t('login.submit')}
              </Button>
            </form>
          )}
        </div>
      </div>
    </>
  )
}
