import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useBrandingStore } from '@/stores/branding'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const setAuth = useAuthStore((state) => state.setAuth)
  const token = useAuthStore((s) => s.token)
  const { siteName, logoImagePath } = useBrandingStore()

  useEffect(() => {
    if (token) {
      navigate('/')
    }
  }, [token, navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      const { token, refresh_token, user } = await login(identifier, password)
      setAuth(token, refresh_token, user)
    } catch {
      setError('Invalid credentials')
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
            {logoImagePath ? (
              <img
                src={`/branding/${logoImagePath}`}
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
              data-testid="login-identifier"
            />
          </div>

          <div className="login__field">
            <Input
              type="password"
              placeholder="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              data-testid="login-password"
            />
          </div>

          {error && <p className="login__error">{error}</p>}

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
