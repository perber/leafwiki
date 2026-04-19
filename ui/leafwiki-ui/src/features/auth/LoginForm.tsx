import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
import { mapApiError } from '@/lib/api/errors'
import { withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import { useSessionStore } from '@/stores/session'
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const user = useSessionStore((s) => s.user)
  const { siteName, logoFile, logoVersion } = useBrandingStore()

  // If already logged in, redirect to home
  if (user) {
    return <Navigate to="/" replace />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      // user already set in the store by the login function
      await login(identifier, password)
      // Redirect to home page after successful login
      navigate('/')
    } catch (err) {
      const mapped = mapApiError(err, 'Login failed')
      toast.error(
        mapped.detail ? `${mapped.message}: ${mapped.detail}` : mapped.message,
      )
    } finally {
      setLoading(false)
    }
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
