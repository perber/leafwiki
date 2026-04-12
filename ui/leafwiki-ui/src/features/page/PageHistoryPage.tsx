import Page404 from '@/components/Page404'
import { buildViewUrl } from '@/lib/routePath'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { type HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect } from 'react'
import { ArrowLeft } from 'lucide-react'
import { useToolbarStore } from '../toolbar/toolbar'
import { toWikiLookupPath } from '@/lib/wikiPath'
import { useLocation, useNavigate } from 'react-router-dom'
import { useProgressbarStore } from '../progressbar/progressbar'
import { useSetPageTitle } from '../viewer/useSetPageTitle'
import { useViewerStore } from '../viewer/viewer'
import { PageHistoryContent } from '@/features/history/PageHistoryContent'
import { usePageHistory } from '@/features/history/pageHistory'

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

  usePageHistory(page?.id ?? null)

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
        label: 'Back to Page',
        hotkey: 'Esc',
        icon: <ArrowLeft size={18} />,
        action: closeHistory,
        variant: 'outline',
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

  return (
    <div className="page-viewer page-history-page">
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
