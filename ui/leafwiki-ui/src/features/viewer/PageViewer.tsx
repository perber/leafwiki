import Page404 from '@/components/Page404'
import { formatRelativeTime } from '@/lib/formatDate'
import {
  DIALOG_COPY_PAGE,
  DIALOG_DELETE_PAGE_CONFIRMATION,
} from '@/lib/registries'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { useDialogsStore } from '@/stores/dialogs'
import { useCallback, useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { BacklinkInfo } from '../links/LinkInfo'
import MarkdownPreview from '../preview/MarkdownPreview'
import { useProgressbarStore } from '../progressbar/progressbar'
import Breadcrumbs from './Breadcrumbs'
import { useScrollToHeadline } from './useScrollToHeadline'
import { useSetPageTitle } from './useSetPageTitle'
import { useToolbarActions } from './useToolbarActions'
import { useViewerStore } from './viewer'

function displayUser(label?: { username: string }) {
  return label?.username || null
}

export default function PageViewer() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const loading = useProgressbarStore((s) => s.loading)
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

  const renderError = () => {
    if (!loading && !page) {
      return <Page404 />
    }
    if (!loading && error) {
      return <p className="page-viewer__error">Error: {error}</p>
    }
    return null
  }

  const editorName = displayUser(page?.metadata?.lastAuthor)

  const updatedRelative = formatRelativeTime(page?.metadata?.updatedAt)
  const createdRelative = formatRelativeTime(page?.metadata?.createdAt)

  const showUpdated = updatedRelative && updatedRelative !== createdRelative

  return (
    <div className="page-viewer">
      <div className="page-viewer__header">
        <Breadcrumbs />
        {page && showUpdated && (
          <div className="page-viewer__metadata">
            <span className="page-viewer__metadata-item">
              Updated{' '}
              {editorName
                ? `by ${editorName} · ${updatedRelative}`
                : updatedRelative}
            </span>
          </div>
        )}
      </div>

      {/* we keep the content also during loading to avoid flickering */}
      {page && !error && (
        <div className="page-viewer__body">
          <article className="page-viewer__content">
            <MarkdownPreview content={page.content} path={page.path} />
          </article>
          <BacklinkInfo />
        </div>
      )}
      {renderError()}
    </div>
  )
}
