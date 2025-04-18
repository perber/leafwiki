import { getPageByPath } from '@/lib/api'
// import "highlight.js/styles/github.css"
import { MarkdownLink } from '@/components/MarkdownLink'
import { usePageToolbar } from '@/components/PageToolbarContext'
import React, { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { useLocation } from 'react-router-dom'
import rehypeHighlight from 'rehype-highlight'
import remarkGfm from 'remark-gfm'
import { DeletePageDialog } from './DeletePageDialog'
import { EditPageButton } from './EditPageButton'

export default function PageViewer() {
  const { pathname } = useLocation()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState<any>(null)
  const { setContent, clearContent } = usePageToolbar()

  useEffect(() => {
    setLoading(true)
    setError(null)

    const path = pathname.slice(1) // remove leading /

    getPageByPath(path)
      .then(setPage)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [pathname])

  useEffect(() => {
    if (!page) return

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
  }, [page, setContent])

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
      <article className="prose prose-lg max-w-none leading-relaxed [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0">
        <ReactMarkdown
          children={page.content}
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeHighlight]}
          components={{
            a: MarkdownLink,
          }}
        />
      </article>
    </>
  )
}
