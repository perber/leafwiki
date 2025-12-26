import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api/auth'
import { useSessionStore } from '@/stores/session'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const user = useSessionStore((s) => s.user)

  useEffect(() => {
    if (user) {
      navigate('/')
    }
  }, [user, navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      // user already set in the store by the login function
      await login(identifier, password)
    } catch {
      setError('Invalid credentials')
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
              name='identifier'
              autoComplete='username'
              data-testid="login-identifier"
              spellCheck={false}
            />
          </div>

          <div className="login__field">
            <Input
              type="password"
              placeholder="Password"
              name='password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              data-testid="login-password"
              autoComplete='current-password'
              spellCheck={false}
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
