import Page404 from '@/components/Page404'
import { getPermalinkTarget } from '@/lib/api/pages'
import { ApiError } from '@/lib/api/auth'
import { useProgressbarStore } from '@/features/progressbar/progressbar'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

export default function PermalinkRedirect() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const setLoading = useProgressbarStore((s) => s.setLoading)
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    const resolve = async () => {
      if (!id) {
        setNotFound(true)
        return
      }

      setLoading(true)
      setNotFound(false)
      setError(null)

      try {
        const target = await getPermalinkTarget(id)
        if (!active) return

        navigate(target.path ? `/${target.path}` : '/', { replace: true })
      } catch (err) {
        if (!active) return

        if (err instanceof ApiError && err.status === 404) {
          setNotFound(true)
          return
        }

        if (err instanceof Error) {
          setError(err.message)
          return
        }

        setError('An unknown error occurred')
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void resolve()

    return () => {
      active = false
      setLoading(false)
    }
  }, [id, navigate, setLoading])

  if (notFound) {
    return <Page404 />
  }

  if (error) {
    return <p className="page-viewer__error">Error: {error}</p>
  }

  return null
}
