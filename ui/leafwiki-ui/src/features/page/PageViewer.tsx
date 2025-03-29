import { getPageByPath } from "@/lib/api"
import { useEffect, useState } from "react"
import { useLocation } from "react-router-dom"

export default function PageViewer() {
  const { pathname } = useLocation()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState<any>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)

    const path = pathname.slice(1) // remove leading /

    getPageByPath(path)
      .then(setPage)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [pathname])

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error) return <p className="text-sm text-red-500">Error: {error}</p>
  if (!page) return <p className="text-sm text-gray-500">No page found</p>
  return (
    <article className="prose max-w-none">
      <h1>{page.title}</h1>
      <pre className="text-xs text-gray-500">Slug: {page.slug}</pre>
      <article className="mt-6 whitespace-pre-wrap">{page.content}</article>
    </article>
  )
}
