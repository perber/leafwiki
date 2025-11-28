import Page404 from '@/components/Page404'
import {
  DIALOG_COPY_PAGE,
  DIALOG_DELETE_PAGE_CONFIRMATION,
} from '@/lib/registries'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { useDialogsStore } from '@/stores/dialogs'
import { useCallback, useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import MarkdownPreview from '../preview/MarkdownPreview'
import Breadcrumbs from './Breadcrumbs'
import { useScrollToHeadline } from './useScrollToHeadline'
import { useSetPageTitle } from './useSetPageTitle'
import { useToolbarActions } from './useToolbarActions'
import { useViewerStore } from './viewer'

export default function PageViewer() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const loading = useViewerStore((s) => s.loading)
  const error = useViewerStore((s) => s.error)
  const page = useViewerStore((s) => s.page)
  const loadPageData = useViewerStore((s) => s.loadPageData)

  const actions = {
    printPage: useCallback(() => {
      window.print()
    }, []),
    editPage: useCallback(() => {
      navigate(`/e/${page?.path || ''}`)
    }, [page?.path, navigate]),
    deletePage: useCallback(() => {
      const redirectUrl = page?.path.split('/').slice(0, -1).join('/')
      openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, {
        pageId: page?.id,
        redirectUrl,
      })
    }, [page, openDialog]),
    copyPage: useCallback(() => {
      if (!page) return
      openDialog(DIALOG_COPY_PAGE, { sourcePage: page })
    }, [page, openDialog]),
  }

  useScrollRestoration(pathname, loading)
  useScrollToHeadline({ content: page?.content || '', isLoading: loading })
  useToolbarActions(actions)
  useSetPageTitle({ page })

  useEffect(() => {
    const path = pathname.slice(1) // remove leading /
    loadPageData?.(path)
  }, [pathname, loadPageData])

  return (
    <div className="p-6">
      <div>
        <Breadcrumbs />
      </div>
      {/* we keep the content also during loading to avoid flickering */}
      {page && !error && (
        <article className="prose prose-base mt-6 max-w-none leading-relaxed [&_img]:h-auto [&_img]:max-w-full [&_li]:leading-snug [&_ol_ol]:mt-0 [&_ol_ol]:mb-0 [&_ol_ul]:mt-0 [&_ul_ol]:mb-0 [&_ul_ul]:mt-0 [&_ul_ul]:mb-0 [&_ul>li::marker]:text-gray-800">
          <MarkdownPreview content={page.content} />
        </article>
      )}
      {/* We show the 404 page only when not loading and no page is found */}
      {!loading && !page && (
        <div>
          <Page404 />
        </div>
      )}
    </div>
  )
}
