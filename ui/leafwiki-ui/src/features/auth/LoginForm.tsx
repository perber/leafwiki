import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
import { mapApiError } from '@/lib/api/errors'
import { safeOAuthAuthorizeReturnTo, withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import { useSessionStore } from '@/stores/session'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const user = useSessionStore((s) => s.user)
  const { siteName, logoFile, logoVersion } = useBrandingStore()
  const oauthReturnTo = useMemo(
    () => safeOAuthAuthorizeReturnTo(searchParams.get('returnTo')),
    [searchParams],
  )

  useEffect(() => {
    if (!user) return
    if (oauthReturnTo) {
      window.location.assign(oauthReturnTo)
      return
    }
    navigate('/', { replace: true })
  }, [navigate, oauthReturnTo, user])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      // user already set in the store by the login function
      await login(identifier, password)
      if (oauthReturnTo) {
        window.location.assign(oauthReturnTo)
        return
      }
      navigate('/')
    } catch (err) {
      const mapped = mapApiError(err, 'Login failed')
      toast.error(mapped.message)
    } finally {
      setLoading(false)
    }
  }

  if (user) {
    return null
  }

  return (
    <>
      <title>Login - {siteName}</title>
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
              placeholder="Username or Email"
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
              placeholder="Password"
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
            {loading ? 'Logging in...' : 'Login'}
          </Button>
        </form>
      </div>
    </>
  )
}
