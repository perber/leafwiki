import { Button } from '@/components/ui/button'
import { ListViewItem } from '@/components/ListView'
import { mapApiError, type ApiUiError } from '@/lib/api/errors'
import { type Page } from '@/lib/api/pages'
import { restoreRevision, type Revision } from '@/lib/api/revisions'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { buildHistoryUrl } from '@/lib/routePath'
import { useDateTimeFormat } from '@/lib/useDateTimeFormat'
import { useIsMobile } from '@/lib/useIsMobile'
import { useTreeStore } from '@/stores/tree'
import {
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { FileText, History, Loader2 } from 'lucide-react'
import { useLinkStatusStore } from '../links/linkStatusStore'
import { useViewerStore } from '../viewer/viewer'
import { confirmRestoreRevision } from './restoreRevisionDialogState'
import {
  type HistoryTab,
  loadMorePageHistory,
  reloadPageHistory,
  usePageHistoryStore,
} from './pageHistory'
import {
  AssetsPanel,
  ChangesPanel,
  PreviewPanel,
  RawTextPanel,
} from './RevisionDetailPanels'

export type PageHistoryContentProps = {
  pageId: string
  pageTitle: string
  pageSlug?: string
  testidPrefix?: string
}

const DEFAULT_HISTORY_LIST_WIDTH = 345
const MIN_HISTORY_LIST_WIDTH = 220
const MAX_HISTORY_LIST_WIDTH = 800

function getInitialHistoryListWidth() {
  if (typeof window === 'undefined') return DEFAULT_HISTORY_LIST_WIDTH

  const storedValue = window.localStorage.getItem('leafwiki-history-list-width')
  const parsed = Number.parseInt(storedValue ?? '', 10)

  if (Number.isNaN(parsed)) return DEFAULT_HISTORY_LIST_WIDTH

  return Math.min(
    MAX_HISTORY_LIST_WIDTH,
    Math.max(MIN_HISTORY_LIST_WIDTH, parsed),
  )
}

// --- Revision list types and helpers ---
// Helpers that need translation or the user's date-format preference are
// defined inside PageHistoryContent (or the relevant detail sub-component)
// via useTranslation() / useDateTimeFormat(), so they stay reactive to a
// language or format change while the view is mounted. Only preference-free
// pure helpers live at module scope.

type RevisionGroup = {
  label: string
  revisions: Revision[]
}

function getPathLeaf(path: string) {
  const segments = path.split('/').filter(Boolean)
  return (segments[segments.length - 1] ?? path) || '/'
}

function ErrorNotice({ error }: { error: ApiUiError }) {
  return (
    <div className="page-history__error-notice">
      <div className="page-history__error-title">{error.message}</div>
    </div>
  )
}

function MetaChip({ children }: { children: ReactNode }) {
  return <span className="page-history__meta-chip">{children}</span>
}

function ChangeChip({
  label,
  from,
  to,
}: {
  label: string
  from: string
  to: string
}) {
  return (
    <div className="page-history__change-chip">
      <span className="page-history__change-chip-label">{label}</span>
      <span className="page-history__change-chip-value">
        <span className="page-history__change-chip-from">{from}</span>
        <span className="page-history__change-chip-arrow" aria-hidden="true">
          →
        </span>
        <span className="page-history__change-chip-to">{to}</span>
      </span>
    </div>
  )
}

function RevisionBadge({
  children,
  testId,
}: {
  children: ReactNode
  testId?: string
}) {
  return (
    <span className="history-sidebar__badge" data-testid={testId}>
      {children}
    </span>
  )
}

function EmptyState({ title, message }: { title: string; message: string }) {
  return (
    <div className="page-history__empty-state">
      <div className="page-history__empty-state-title">{title}</div>
      <div className="page-history__empty-state-message">{message}</div>
    </div>
  )
}

export function PageHistoryContent({
  pageId,
  pageTitle,
  pageSlug,
  testidPrefix = 'page-history',
}: PageHistoryContentProps) {
  const { t } = useTranslation('history')
  const { formatDateOnly, formatTimeOnly, formatDateTime, formatRelativeTime } =
    useDateTimeFormat()
  const navigate = useNavigate()
  const isMobile = useIsMobile()
  const revisions = usePageHistoryStore((state) => state.revisions)
  const selectedRevisionId = usePageHistoryStore(
    (state) => state.selectedRevisionId,
  )
  const snapshot = usePageHistoryStore((state) => state.snapshot)
  const comparison = usePageHistoryStore((state) => state.comparison)
  const activeTab = usePageHistoryStore((state) => state.activeTab)
  const listLoading = usePageHistoryStore((state) => state.listLoading)
  const previewLoading = usePageHistoryStore((state) => state.previewLoading)
  const compareLoading = usePageHistoryStore((state) => state.compareLoading)
  const listError = usePageHistoryStore((state) => state.listError)
  const latestRevisionId = usePageHistoryStore(
    (state) => state.latestRevisionId,
  )
  const previewError = usePageHistoryStore((state) => state.previewError)
  const setActiveTab = usePageHistoryStore((state) => state.setActiveTab)
  const nextCursor = usePageHistoryStore((state) => state.nextCursor)
  const loadingMore = usePageHistoryStore((state) => state.loadingMore)
  const selectRevision = usePageHistoryStore((state) => state.selectRevision)
  const [restoreLoading, setRestoreLoading] = useState(false)
  const [listWidth, setListWidth] = useState(getInitialHistoryListWidth)
  const [isResizingList, setIsResizingList] = useState(false)
  const [isListResizeHovered, setIsListResizeHovered] = useState(false)
  const [mobileListVisible, setMobileListVisible] = useState(true)
  const liveListWidthRef = useRef(listWidth)
  const resizeHandlersRef = useRef<{
    onMouseMove: (event: MouseEvent) => void
    onMouseUp: () => void
  } | null>(null)

  const revisionTriggerLabel = useCallback(
    (type: string) => {
      switch (type) {
        case 'content_update':
          return t('trigger.contentUpdate')
        case 'asset_update':
          return t('trigger.assetUpdate')
        case 'structure_update':
          return t('trigger.structureUpdate')
        case 'restore':
          return t('trigger.restore')
        case 'delete':
          return t('trigger.delete')
        default:
          return t('trigger.generic', { type })
      }
    },
    [t],
  )

  const revisionTitle = (revision: Revision) =>
    !revision.createdAt
      ? t('common.unknownTime')
      : formatTimeOnly(revision.createdAt) || revision.createdAt

  const authorLabel = (revision: Revision) =>
    revision.author?.username || revision.authorId || t('common.unknown')

  const formatTimestamp = (value?: string) =>
    value ? formatDateTime(value) || value : ''

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const groupedRevisions = useMemo(() => {
    const groups: RevisionGroup[] = []
    revisions.forEach((revision) => {
      const label = revision.createdAt
        ? formatDateOnly(revision.createdAt) || t('common.unknown')
        : t('common.unknown')
      const lastGroup = groups[groups.length - 1]
      if (!lastGroup || lastGroup.label !== label) {
        groups.push({ label, revisions: [revision] })
        return
      }
      lastGroup.revisions.push(revision)
    })
    return groups
  }, [revisions, t, formatDateOnly])
  const isSelectedRevisionLatest =
    !!selectedRevision && selectedRevision.id === latestRevisionId

  const chips = useMemo(() => {
    if (!selectedRevision) return []

    const result = [
      t('chips.revisionSlug', { slug: selectedRevision.slug || '/' }),
      getPathLeaf(selectedRevision.path),
      revisionTriggerLabel(selectedRevision.type),
    ]

    if (pageSlug && pageSlug !== selectedRevision.slug) {
      result.unshift(t('chips.currentSlug', { slug: pageSlug }))
    }

    if (comparison) {
      result.push(
        t('chips.assetChangesCount', { count: comparison.assetChanges.length }),
      )
    } else if (snapshot) {
      result.push(t('chips.assetsCount', { count: snapshot.assets.length }))
    }

    return result
  }, [
    comparison,
    pageSlug,
    selectedRevision,
    snapshot,
    t,
    revisionTriggerLabel,
  ])

  const structureChanges = useMemo(() => {
    if (!comparison) return []

    const changes: Array<{ label: string; from: string; to: string }> = []

    if (comparison.base.revision?.title !== comparison.target.revision?.title) {
      changes.push({
        label: t('header.titleLabel'),
        from: comparison.base.revision?.title || t('header.emptyValue'),
        to: comparison.target.revision?.title || t('header.emptyValue'),
      })
    }

    if (comparison.base.revision?.slug !== comparison.target.revision?.slug) {
      changes.push({
        label: t('header.slugLabel'),
        from: comparison.base.revision?.slug || t('header.emptyValue'),
        to: comparison.target.revision?.slug || t('header.emptyValue'),
      })
    }

    return changes
  }, [comparison, t])

  // Preview is first and the default active tab so users immediately see the
  // rendered content of the selected revision without an extra click.
  const tabs: { id: HistoryTab; label: string }[] = [
    { id: 'preview', label: t('tabs.preview') },
    { id: 'changes', label: t('tabs.changes') },
    { id: 'raw', label: t('tabs.raw') },
    { id: 'assets', label: t('tabs.assets') },
  ]

  const detailLoading =
    activeTab === 'changes' || activeTab === 'assets'
      ? compareLoading
      : previewLoading

  useEffect(() => {
    liveListWidthRef.current = listWidth
  }, [listWidth])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem(
      'leafwiki-history-list-width',
      String(listWidth),
    )
  }, [listWidth])

  useEffect(() => {
    if (!isMobile) {
      setMobileListVisible(true)
      return
    }

    if (selectedRevisionId) {
      setMobileListVisible(false)
    }
  }, [isMobile, selectedRevisionId])

  useEffect(() => {
    if (!isResizingList || !resizeHandlersRef.current) return

    const { onMouseMove, onMouseUp } = resizeHandlersRef.current
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)

    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
  }, [isResizingList])

  useEffect(
    () => () => {
      if (!resizeHandlersRef.current) return

      const { onMouseMove, onMouseUp } = resizeHandlersRef.current
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    },
    [],
  )

  const handleRestore = async () => {
    if (!selectedRevision || isSelectedRevisionLatest || restoreLoading) return

    const confirmed = await confirmRestoreRevision(
      selectedRevision,
      pageSlug || '',
    )
    if (confirmed !== true) return

    setRestoreLoading(true)
    try {
      const restoredPage = (await restoreRevision(
        pageId,
        selectedRevision.id,
      )) as Page

      await useTreeStore.getState().reloadTree()
      await useViewerStore.getState().loadPageData(restoredPage.path)

      const viewerPageID = useViewerStore.getState().page?.id
      if (viewerPageID) {
        await useLinkStatusStore.getState().fetchLinkStatusForPage(viewerPageID)
      } else {
        useLinkStatusStore.getState().clear()
      }

      await reloadPageHistory(pageId)
      navigate(buildHistoryUrl(restoredPage.path), {
        replace: true,
        state: createNavigationVisitState(),
      })
      toast.success(t('toasts.restoreSuccess'))
    } catch (err) {
      const mapped = mapApiError(err, t('toasts.restoreErrorFallback'))
      toast.error(mapped.message)
    } finally {
      setRestoreLoading(false)
    }
  }

  const handleListResize = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (isMobile) return

    event.preventDefault()
    event.stopPropagation()

    const startX = event.clientX
    const startWidth = listWidth

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX
      const viewportWidth = window.innerWidth
      const maxWidth = Math.min(viewportWidth - 320, MAX_HISTORY_LIST_WIDTH)
      const nextWidth = Math.min(
        maxWidth,
        Math.max(MIN_HISTORY_LIST_WIDTH, startWidth + delta),
      )

      liveListWidthRef.current = nextWidth
      setListWidth(nextWidth)
    }

    const onMouseUp = () => {
      setListWidth(liveListWidthRef.current)
      setIsResizingList(false)
      setIsListResizeHovered(false)
      resizeHandlersRef.current = null
    }

    resizeHandlersRef.current = { onMouseMove, onMouseUp }
    setIsResizingList(true)
  }

  const renderDetailContent = () => {
    if (listLoading) {
      return (
        <div className="page-history__loading-state">
          {t('detail.loadingHistory')}
        </div>
      )
    }

    if (listError) {
      return <ErrorNotice error={listError} />
    }

    if (!selectedRevision) {
      return (
        <EmptyState
          title={t('detail.noRevisionSelectedTitle')}
          message={t('detail.noRevisionSelectedMessage')}
        />
      )
    }

    if (detailLoading && !comparison && !snapshot) {
      return (
        <div className="page-history__loading-state">
          {activeTab === 'changes' || activeTab === 'assets'
            ? t('detail.loadingDiff')
            : t('detail.loadingPreview')}
        </div>
      )
    }

    if (previewError) {
      return <ErrorNotice error={previewError} />
    }

    if (activeTab === 'preview') {
      return snapshot ? (
        <PreviewPanel snapshot={snapshot} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          {t('detail.noPreview')}
        </div>
      )
    }

    if (activeTab === 'changes') {
      if (isSelectedRevisionLatest) {
        return (
          <div className="page-history__empty-message page-history__empty-message--padded">
            {t('detail.noChangesFromCurrent')}
          </div>
        )
      }

      return comparison ? (
        <ChangesPanel comparison={comparison} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          {t('detail.noComparisonData')}
        </div>
      )
    }

    if (activeTab === 'raw') {
      return snapshot ? (
        <RawTextPanel snapshot={snapshot} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          {t('detail.noRawText')}
        </div>
      )
    }

    return snapshot ? (
      <AssetsPanel snapshot={snapshot} />
    ) : (
      <div className="page-history__empty-message page-history__empty-message--padded">
        {t('detail.noAssetData')}
      </div>
    )
  }

  const renderRevisionList = () => {
    if (listLoading) {
      return (
        <div className="page-history__list-status">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t('list.loading')}
        </div>
      )
    }

    if (listError) {
      return (
        <div className="page-history__list-status">
          <ErrorNotice error={listError} />
        </div>
      )
    }

    if (revisions.length === 0) {
      return (
        <div className="page-history__list-status">
          {latestRevisionId
            ? t('list.emptyWithLatest')
            : t('list.emptyNoLatest')}
        </div>
      )
    }

    return (
      <>
        {groupedRevisions.map((group) => (
          <div key={group.label} className="history-sidebar__group">
            <div className="history-sidebar__group-label">{group.label}</div>
            {group.revisions.map((revision) => {
              const selected = revision.id === selectedRevisionId

              return (
                <ListViewItem
                  key={revision.id}
                  active={selected}
                  className="history-sidebar__item"
                  onClick={() => {
                    selectRevision(revision.id)
                    if (isMobile) {
                      setMobileListVisible(false)
                    }
                  }}
                  testId={`history-sidebar-revision-${revision.id}`}
                >
                  <div className="history-sidebar__item-heading">
                    <div className="history-sidebar__item-title">
                      {revisionTitle(revision)}
                    </div>
                    {revision.id === latestRevisionId ? (
                      <RevisionBadge
                        testId={`history-sidebar-revision-current-badge-${revision.id}`}
                      >
                        {t('list.activeVersionBadge')}
                      </RevisionBadge>
                    ) : null}
                  </div>
                  <div className="history-sidebar__item-meta">
                    {authorLabel(revision)}
                  </div>
                </ListViewItem>
              )
            })}
          </div>
        ))}

        {nextCursor ? (
          <div className="history-sidebar__load-more">
            <Button
              variant="outline"
              className="w-full"
              onClick={() => void loadMorePageHistory()}
              disabled={loadingMore}
            >
              {loadingMore ? t('list.loadMoreLoading') : t('list.loadMore')}
            </Button>
          </div>
        ) : null}
      </>
    )
  }

  return (
    <div className="page-history" data-testid={`${testidPrefix}-content`}>
      {isMobile && (
        <div className="markdown-editor__tabs" role="tablist">
          {(
            [
              {
                id: 'list',
                label: t('tabs.mobileRevisions'),
                icon: <History size={16} />,
              },
              {
                id: 'detail',
                label: t('tabs.mobileDetails'),
                icon: <FileText size={16} />,
              },
            ] as const
          ).map((tab) => {
            const active =
              tab.id === 'list' ? mobileListVisible : !mobileListVisible
            return (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setMobileListVisible(tab.id === 'list')}
                className={
                  active
                    ? 'markdown-editor__tab-button markdown-editor__tab-button--active'
                    : 'markdown-editor__tab-button markdown-editor__tab-button--inactive'
                }
              >
                {tab.icon}
                {tab.label}
              </button>
            )
          })}
        </div>
      )}
      <div className="page-history__workspace">
        {(mobileListVisible || !isMobile) && (
          <>
            <div
              className="page-history__panel page-history__panel--list"
              data-testid={`${testidPrefix}-list`}
              style={isMobile ? undefined : { width: `${listWidth}px` }}
            >
              <div className="page-history__list-header">
                <div className="page-history__list-title">
                  <History className="h-4 w-4" />
                  {t('list.title')}
                </div>
              </div>
              <div className="page-history__list-scroll custom-scrollbar">
                {renderRevisionList()}
              </div>
            </div>

            {!isMobile && (
              <div
                className="page-history__panel-resizer"
                onMouseDown={handleListResize}
                onMouseEnter={() => setIsListResizeHovered(true)}
                onMouseLeave={() => {
                  if (!resizeHandlersRef.current) setIsListResizeHovered(false)
                }}
                role="separator"
                aria-orientation="vertical"
                aria-label={t('list.resizeAriaLabel')}
                data-testid={`${testidPrefix}-list-resize-handle`}
              >
                <div
                  className={`page-history__panel-resize-handle ${
                    isListResizeHovered || isResizingList
                      ? 'page-history__panel-resize-handle--hover'
                      : 'page-history__panel-resize-handle--default'
                  }`}
                />
              </div>
            )}
          </>
        )}

        {/* Right panel: selected revision detail */}
        <div
          className={`page-history__panel page-history__panel--detail ${
            isMobile && mobileListVisible
              ? 'page-history__panel--detail-hidden'
              : ''
          }`}
        >
          <div className="page-history__header">
            <div className="page-history__header-copy">
              <div className="page-history__header-title">
                {selectedRevision?.title || pageTitle}
              </div>
              {selectedRevision ? (
                <div className="page-history__header-subtitle">
                  {t('header.revisionBy', {
                    author: authorLabel(selectedRevision),
                    time:
                      formatRelativeTime(selectedRevision.createdAt) ||
                      formatTimestamp(selectedRevision.createdAt),
                  })}
                </div>
              ) : null}
              {selectedRevision ? (
                <div className="page-history__meta-chips">
                  {chips.map((chip) => (
                    <MetaChip key={chip}>{chip}</MetaChip>
                  ))}
                </div>
              ) : null}
              {structureChanges.length > 0 ? (
                <div
                  className="page-history__change-chips"
                  data-testid={`${testidPrefix}-structure-changes`}
                >
                  {structureChanges.map((change) => (
                    <ChangeChip
                      key={change.label}
                      label={change.label}
                      from={change.from}
                      to={change.to}
                    />
                  ))}
                </div>
              ) : null}
            </div>

            <div className="page-history__actions">
              <Button
                variant="default"
                disabled={
                  !selectedRevision ||
                  isSelectedRevisionLatest ||
                  restoreLoading
                }
                onClick={() => void handleRestore()}
                data-testid={`${testidPrefix}-restore`}
              >
                {restoreLoading
                  ? t('header.restoreLoading')
                  : isSelectedRevisionLatest
                    ? t('header.currentVersion')
                    : t('header.restore')}
              </Button>
            </div>
          </div>

          <div className="page-history__tabs" role="tablist">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={activeTab === tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={
                  activeTab === tab.id
                    ? 'page-history__tab-button page-history__tab-button--active'
                    : 'page-history__tab-button page-history__tab-button--inactive'
                }
                data-testid={`${testidPrefix}-${tab.id}-tab`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="page-history__detail-content custom-scrollbar">
            {renderDetailContent()}
          </div>
        </div>
      </div>
    </div>
  )
}
