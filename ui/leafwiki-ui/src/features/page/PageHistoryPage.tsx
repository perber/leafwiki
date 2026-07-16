import Page404 from '@/components/Page404'
import { buildViewUrl } from '@/lib/routePath'
import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
} from '@/lib/shortcuts/shortcutCatalog'
import {
  createNavigationVisitState,
  getNavigationVisitKey,
} from '@/lib/navigationVisit'
import { useScrollRestoration } from '@/lib/useScrollRestoration'
import { type HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowLeft } from 'lucide-react'
import { useToolbarStore } from '../toolbar/toolbarStore'
import { getWikiTargetRoutePath, toWikiLookupPath } from '@/lib/wikiPath'
import { useLocation, useNavigate } from 'react-router-dom'
import { useProgressbarStore } from '../progressbar/progressbarStore'
import { useSetPageTitle } from '../viewer/useSetPageTitle'
import { useViewerStore } from '../viewer/viewer'
import { PageHistoryContent } from '@/features/history/PageHistoryContent'
import { usePageHistory } from '@/features/history/pageHistory'

export default function PageHistoryPage() {
  const { t } = useTranslation('page')
  const { t: tCommon } = useTranslation('common')
  const location = useLocation()
  const { pathname } = location
  const navigate = useNavigate()
  const openNode = useTreeStore((state) => state.openNode)
  const setToolbarButtons = useToolbarStore((state) => state.setButtons)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)
  const loading = useProgressbarStore((s) => s.loading)
  const error = useViewerStore((s) => s.error)
  const notFound = useViewerStore((s) => s.notFound)
  const page = useViewerStore((s) => s.page)
  const loadPageData = useViewerStore((s) => s.loadPageData)
  const isMacOS =
    typeof navigator !== 'undefined' &&
    /Mac|iPhone|iPad|iPod/.test(navigator.platform)

  usePageHistory(page?.id ?? null)

  const closeHistory = useCallback(() => {
    navigate(buildViewUrl(page?.path || pathname), {
      state: createNavigationVisitState(),
    })
  }, [navigate, page?.path, pathname])

  useScrollRestoration(getNavigationVisitKey(location), loading)
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
        label: t('historyPage.backToPage'),
        hotkey: getShortcutDisplayLabel('history.page.close', isMacOS),
        icon: <ArrowLeft size={18} />,
        action: closeHistory,
        variant: 'outline',
      },
    ])

    const closeHotkey: HotKeyDefinition = createHotkeyDefinition(
      'history.page.close',
      closeHistory,
    )

    registerHotkey(closeHotkey)

    return () => {
      setToolbarButtons([])
      unregisterHotkey(closeHotkey.keyCombo)
    }
  }, [
    closeHistory,
    isMacOS,
    registerHotkey,
    setToolbarButtons,
    unregisterHotkey,
    t,
  ])

  const renderError = () => {
    if (!loading && notFound) {
      return <Page404 targetPath={getWikiTargetRoutePath(pathname)} />
    }
    if (!loading && error) {
      return <p className="page-viewer__error">{tCommon('errorPrefix')} {error}</p>
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
              pageSlug={page.slug}
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
