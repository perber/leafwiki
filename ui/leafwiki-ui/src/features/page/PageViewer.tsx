/* eslint-disable react-hooks/set-state-in-effect */
import Breadcrumbs from '@/components/Breadcrumbs'
import { Button } from '@/components/ui/button'
import { usePageToolbar } from '@/components/usePageToolbar'
import { getPageByPath, Page } from '@/lib/api/pages'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'
import { Fragment, useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import MarkdownPreview from '../preview/MarkdownPreview'
import { CopyPageButton } from './CopyPageButton'
import { DeletePageButton } from './DeletePageButton'
import { EditPageButton } from './EditPageButton'
import { PrintPageButton } from './PrintPageButton'

export default function PageViewer() {
  const { pathname } = useLocation()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const user = useAuthStore((s) => s.user)

  const readOnlyMode = useIsReadOnly()

  const [page, setPage] = useState<Page | null>(null)
  const { setContent, clearContent } = usePageToolbar()

  useScrollRestoration(pathname, loading)

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
      <Fragment key="viewing">
        <DeletePageButton pageId={page.id} redirectUrl={redirectUrl} />
        <CopyPageButton sourcePage={page} />
        <PrintPageButton />
        <EditPageButton path={page.path} />
      </Fragment>,
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
  if (!page) {
    return (
      <div>
        <h1 className="text-1xl mb-2 font-bold text-red-500">Page Not Found</h1>
        <p className="text-sm text-gray-500">
          The page you are looking for does not exist.
        </p>
        {user && (
          <>
            <p className="text-sm text-gray-500">
              Create the page by clicking the button below.
            </p>
            <Button
              className="mt-4"
              onClick={() =>
                openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
                  initialPath: pathname,
                  readOnlyPath: true,
                  forwardToEditMode: true,
                })
              }
              variant={'outline'}
            >
              Create Page
            </Button>
          </>
        )}
      </div>
    )
  }
  if (error) {
    return <p className="text-sm text-red-500">Error: {error}</p>
  }

  return (
    <>
      <div className="mb-6">
        <Breadcrumbs />
      </div>
      <article className="page-view prose prose-base max-w-none leading-relaxed [&_img]:h-auto [&_img]:max-w-full [&_li]:leading-snug [&_ol_ol]:mt-0 [&_ol_ol]:mb-0 [&_ol_ul]:mt-0 [&_ul_ol]:mb-0 [&_ul_ul]:mt-0 [&_ul_ul]:mb-0 [&_ul>li::marker]:text-gray-800">
        <MarkdownPreview content={page.content} />
      </article>
    </>
  )
}
