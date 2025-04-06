import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { login } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

export default function LoginForm() {
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const navigate = useNavigate()
  const setAuth = useAuthStore((state) => state.setAuth)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      const { token, refresh_token, user } = await login(identifier, password)
      setAuth(token, refresh_token, user)
      navigate('/') // redirect after login
    } catch (err: any) {
      setError('Invalid credentials')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="h-screen w-96 border-r border-gray-200 bg-white font-sans text-gray-900 shadow-md">
      <form onSubmit={handleSubmit} className="w-full p-4">
        <h1 className="mb-4 text-xl font-bold">🌿 LeafWiki</h1>

        <div className="mb-4">
          <Input
            type="text"
            placeholder="Username or Email"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            required
          />
        </div>

        <div className="mb-4">
          <Input
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>

        {error && <p className="mb-4 text-sm text-red-600">{error}</p>}

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Logging in...' : 'Login'}
        </Button>
      </form>
    </div>
  )
}
