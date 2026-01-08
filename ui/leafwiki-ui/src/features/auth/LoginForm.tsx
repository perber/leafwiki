import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
import { useSessionStore } from '@/stores/session'
import { useBrandingStore } from '@/stores/branding'
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const user = useSessionStore((s) => s.user)

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
      toast.error(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <title>Login - LeafWiki</title>
      <div className="login">
        <form onSubmit={handleSubmit} className="login__form">
          <h1 className="login__title">🌿 LeafWiki</h1>

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
