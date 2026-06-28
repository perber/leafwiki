import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      // user already set in the store by the login function
      await login(identifier, password)
      // Restore the originally requested page after successful login.
      navigate(redirectTo || '/', { replace: true })
    } catch (err) {
      const mapped = mapApiError(err, t('login.errorFallback'))
      toast.error(mapped.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <title>{t('login.pageTitle', { siteName })}</title>
      <div className="login">
        <form onSubmit={handleSubmit} className="login__form">
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
