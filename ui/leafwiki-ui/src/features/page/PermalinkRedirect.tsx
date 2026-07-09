import Page404 from '@/components/Page404'
import { getPermalinkTarget } from '@/lib/api/pages'
import { isPageNotFoundError } from '@/lib/api/errors'
import { useProgressbarStore } from '@/features/progressbar/progressbarStore'
import i18next from '@/lib/i18n'
import { useEffect, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'

export default function PermalinkRedirect() {
  const location = useLocation()
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

        navigate(target.path ? `/${target.path}` : '/', {
          replace: true,
          state: location.state,
        })
      } catch (err) {
        if (!active) return

        if (isPageNotFoundError(err)) {
          setNotFound(true)
          return
        }

        if (err instanceof Error) {
          setError(err.message)
          return
        }

        setError(i18next.t('unknownError', { ns: 'common' }))
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
  }, [id, location.state, navigate, setLoading])

  if (notFound) {
    return <Page404 />
  }

  if (error) {
    return <p className="page-viewer__error">Error: {error}</p>
  }

  return null
}
