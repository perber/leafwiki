import Page404 from '@/components/Page404'
import { formatRelativeTime } from '@/lib/formatDate'
import { buildViewUrl } from '@/lib/routePath'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { type HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect } from 'react'
import { X } from 'lucide-react'
import { useToolbarStore } from '../toolbar/toolbar'
import { toWikiLookupPath } from '@/lib/wikiPath'
import { useLocation, useNavigate } from 'react-router-dom'
import Breadcrumbs from '../viewer/Breadcrumbs'
import { useProgressbarStore } from '../progressbar/progressbar'
import { useSetPageTitle } from '../viewer/useSetPageTitle'
import { useViewerStore } from '../viewer/viewer'
import { PageHistoryContent } from './PageHistoryContent'

function displayUser(label?: { username: string }) {
  return label?.username || null
}

export default function PageHistoryPage() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const openNode = useTreeStore((state) => state.openNode)
  const setToolbarButtons = useToolbarStore((state) => state.setButtons)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)
  const loading = useProgressbarStore((s) => s.loading)
  const error = useViewerStore((s) => s.error)
  const page = useViewerStore((s) => s.page)
  const loadPageData = useViewerStore((s) => s.loadPageData)

  const closeHistory = useCallback(() => {
    navigate(buildViewUrl(page?.path || pathname))
  }, [navigate, page?.path, pathname])

  useScrollRestoration(pathname, loading)
  useSetPageTitle({ page })

  useEffect(() => {
    const path = toWikiLookupPath(buildViewUrl(pathname))
    void loadPageData?.(path)
  }, [pathname, loadPageData])

  useEffect(() => {
    if (!page?.id) return
    openNode(page.id)
  }, [openNode, page?.id])

  useEffect(() => {
    setToolbarButtons([
      {
        id: 'close-history',
        label: 'Close History',
        hotkey: 'Esc',
        icon: <X size={18} />,
        action: closeHistory,
        variant: 'destructive',
        className: 'toolbar-button__close-editor',
      },
    ])

    const closeHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['history'],
      action: closeHistory,
    }

    registerHotkey(closeHotkey)

    return () => {
      setToolbarButtons([])
      unregisterHotkey(closeHotkey.keyCombo)
    }
  }, [closeHistory, registerHotkey, setToolbarButtons, unregisterHotkey])

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

  return (
    <div className="page-viewer page-history-page">
      <div className="page-viewer__header">
        <div className="page-history-page__header-main">
          <Breadcrumbs />
          {page && (
            <div className="page-viewer__metadata">
              <span className="page-viewer__metadata-item">
                History
                {updatedRelative
                  ? ` · Updated ${editorName ? `by ${editorName} · ${updatedRelative}` : updatedRelative}`
                  : ''}
              </span>
            </div>
          )}
        </div>
      </div>

      {page && !error ? (
        <div className="page-viewer__body page-history-page__body">
          <article className="page-history-page__content">
            <PageHistoryContent
              pageId={page.id}
              pageTitle={page.title}
              testidPrefix="page-history-page"
            />
          </article>
        </div>
      ) : (
        renderError()
      )}
    </div>
  )
}
