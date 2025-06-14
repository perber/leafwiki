import { getPageByPath } from '@/lib/api'
// import "highlight.js/styles/github.css"
import Breadcrumbs from '@/components/Breadcrumbs'
import { usePageToolbar } from '@/components/usePageToolbar'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import React, { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import MarkdownPreview from '../preview/MarkdownPreview'
import { DeletePageDialog } from './DeletePageDialog'
import { EditPageButton } from './EditPageButton'

export default function PageViewer() {
  const { pathname } = useLocation()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const readOnlyMode = useIsReadOnly()

  interface Page {
    id: string
    path: string
    title: string
    content: string
  }

  const [page, setPage] = useState<Page | null>(null)
  const { setContent, clearContent } = usePageToolbar()

  console.log("changed", pathname)

  useEffect(() => {
    setLoading(true)
    setPage(null)
    setError(null)

    const path = pathname.slice(1) // remove leading /

    getPageByPath(path)
      .then((data) => {
        if (isPage(data)) {
          setPage(data)
        } else {
          throw new Error('Invalid page data')
        }
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))

    function isPage(data: unknown): data is Page {
      return (
        typeof data === 'object' &&
        data !== null &&
        'id' in data &&
        typeof data.id === 'string' &&
        'path' in data &&
        typeof data.path === 'string' &&
        'title' in data &&
        typeof data.title === 'string' &&
        'content' in data &&
        typeof data.content === 'string'
      )
    }
  }, [pathname])

  useEffect(() => {
    if (!page) return
    if (readOnlyMode) {
      return
    }

    const redirectUrl = page.path.split('/').slice(0, -1).join('/')

    setContent(
      <React.Fragment key="viewing">
        <DeletePageDialog pageId={page.id} redirectUrl={redirectUrl} />
        <EditPageButton path={page.path} />
      </React.Fragment>,
    )

    return () => {
      clearContent()
    }
  }, [page, setContent, clearContent, readOnlyMode])

  useEffect(() => {
    if (!page) {
      document.title = 'LeafWiki'
    } else {
      document.title = page.title + ' - LeafWiki'
    }
  }, [page])

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error) return <p className="text-sm text-red-500">Error: {error}</p>
  if (!page) return <p className="text-sm text-gray-500">No page found</p>

  return (
    <>
      <div className="mb-6">
        <Breadcrumbs />
      </div>
      <article className="prose prose-lg max-w-none leading-relaxed [&_img]:h-auto [&_img]:max-w-full [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0">
        <MarkdownPreview content={page.content} />
      </article>
    </>
  )
}
