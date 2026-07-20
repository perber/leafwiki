import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { completeTOTPLogin, login } from '@/lib/api/auth'
import { mapApiError } from '@/lib/api/errors'
import { withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import { useSessionStore } from '@/stores/session'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
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
  const user = useSessionStore((s) => s.user)
  const { siteName, logoFile, logoVersion } = useBrandingStore()
  const redirectTo = getRedirectTo(location.state)

  // If already logged in, redirect to home
  if (user) {
    return <Navigate to={redirectTo || '/'} replace />
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
        <form onSubmit={handleCredentialsSubmit} className="login__form">
          {logoHeader}

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

          <Button
            type="submit"
            className="login__submit"
            disabled={loading}
            data-testid="login-submit"
          >
            {loading ? t('login.submitting') : t('login.submit')}
          </Button>
        </form>
      </div>
    </>
  )
}
