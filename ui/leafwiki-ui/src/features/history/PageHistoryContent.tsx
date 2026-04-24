import { Button } from '@/components/ui/button'
import { ListViewItem } from '@/components/ListView'
import { mapApiError, type ApiUiError } from '@/lib/api/errors'
import { type Page } from '@/lib/api/pages'
import {
  buildRevisionAssetUrl,
  restoreRevision,
  type Revision,
  type RevisionAssetChange,
  type RevisionComparison,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { formatRelativeTime } from '@/lib/formatDate'
import { buildHistoryUrl, withBasePath } from '@/lib/routePath'
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
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import {
  Download,
  ExternalLink,
  FileText,
  History,
  Loader2,
  PanelLeftOpen,
} from 'lucide-react'
import { useLinkStatusStore } from '../links/linkstatus_store'
import { AssetPreviewTooltip } from '../assets/AssetPreviewTooltip'
import MarkdownPreview from '../preview/MarkdownPreview'
import { useViewerStore } from '../viewer/viewer'
import { confirmRestoreRevision } from './restoreRevisionDialog'
import {
  type HistoryTab,
  loadMorePageHistory,
  reloadPageHistory,
  usePageHistoryStore,
} from './pageHistory'

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

type RevisionGroup = {
  label: string
  revisions: Revision[]
}

function groupLabel(value?: string) {
  if (!value) return 'Unknown'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
  }).format(date)
}

function groupRevisions(revisions: Revision[]): RevisionGroup[] {
  const groups: RevisionGroup[] = []

  revisions.forEach((revision) => {
    const label = groupLabel(revision.createdAt)
    const existingGroup = groups[groups.length - 1]

    if (!existingGroup || existingGroup.label !== label) {
      groups.push({ label, revisions: [revision] })
      return
    }

    existingGroup.revisions.push(revision)
  })

  return groups
}

function revisionTitle(revision: Revision) {
  if (!revision.createdAt) return 'Unknown time'

  const date = new Date(revision.createdAt)
  if (Number.isNaN(date.getTime())) return revision.createdAt

  return new Intl.DateTimeFormat(undefined, {
    timeStyle: 'short',
  }).format(date)
}

function revisionMeta(revision: Revision) {
  return revision.author?.username || revision.authorId || 'Unknown'
}

function getPathLeaf(path: string) {
  const segments = path.split('/').filter(Boolean)
  return (segments[segments.length - 1] ?? path) || '/'
}

// --- Diff / detail helpers ---

type DiffLine = {
  kind: 'context' | 'added' | 'removed'
  value: string
  oldLineNumber: number | null
  newLineNumber: number | null
}

type DiffSummary = {
  addedLines: number
  removedLines: number
}

function revisionTriggerLabel(type: string) {
  switch (type) {
    case 'content_update':
      return 'Saved after content update'
    case 'asset_update':
      return 'Saved after asset update'
    case 'structure_update':
      return 'Saved after structure update'
    case 'restore':
      return 'Saved after restore'
    case 'delete':
      return 'Saved before delete'
    default:
      return `Saved as ${type}`
  }
}

function assetChangeLabel(status: RevisionAssetChange['status']) {
  switch (status) {
    case 'added':
      return 'Added'
    case 'removed':
      return 'Removed'
    case 'modified':
      return 'Replaced'
    default:
      return status
  }
}

function displayAuthor(revision: Revision) {
  return revision.author?.username || revision.authorId || 'Unknown'
}

function formatTimestamp(value?: string) {
  if (!value) return ''

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function buildLineDiff(
  baseContent: string,
  targetContent: string,
): {
  lines: DiffLine[]
  summary: DiffSummary
} {
  const baseLines = baseContent.split('\n')
  const targetLines = targetContent.split('\n')
  const rows = baseLines.length
  const cols = targetLines.length
  const matrix = new Uint32Array((rows + 1) * (cols + 1))

  const index = (row: number, col: number) => row * (cols + 1) + col

  for (let row = rows - 1; row >= 0; row -= 1) {
    for (let col = cols - 1; col >= 0; col -= 1) {
      if (baseLines[row] === targetLines[col]) {
        matrix[index(row, col)] = matrix[index(row + 1, col + 1)] + 1
      } else {
        matrix[index(row, col)] = Math.max(
          matrix[index(row + 1, col)],
          matrix[index(row, col + 1)],
        )
      }
    }
  }

  const lines: DiffLine[] = []
  let row = 0
  let col = 0
  let oldLineNumber = 1
  let newLineNumber = 1
  let addedLines = 0
  let removedLines = 0

  while (row < rows && col < cols) {
    if (baseLines[row] === targetLines[col]) {
      lines.push({
        kind: 'context',
        value: baseLines[row],
        oldLineNumber,
        newLineNumber,
      })
      row += 1
      col += 1
      oldLineNumber += 1
      newLineNumber += 1
      continue
    }

    if (matrix[index(row + 1, col)] >= matrix[index(row, col + 1)]) {
      lines.push({
        kind: 'removed',
        value: baseLines[row],
        oldLineNumber,
        newLineNumber: null,
      })
      row += 1
      oldLineNumber += 1
      removedLines += 1
      continue
    }

    lines.push({
      kind: 'added',
      value: targetLines[col],
      oldLineNumber: null,
      newLineNumber,
    })
    col += 1
    newLineNumber += 1
    addedLines += 1
  }

  while (row < rows) {
    lines.push({
      kind: 'removed',
      value: baseLines[row],
      oldLineNumber,
      newLineNumber: null,
    })
    row += 1
    oldLineNumber += 1
    removedLines += 1
  }

  while (col < cols) {
    lines.push({
      kind: 'added',
      value: targetLines[col],
      oldLineNumber: null,
      newLineNumber,
    })
    col += 1
    newLineNumber += 1
    addedLines += 1
  }

  return {
    lines,
    summary: { addedLines, removedLines },
  }
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

function SummaryStat({
  label,
  value,
  emphasized = false,
  tone = 'default',
}: {
  label: string
  value: string
  emphasized?: boolean
  tone?: 'default' | 'added' | 'removed'
}) {
  return (
    <div
      className={`page-history__summary-stat page-history__summary-stat--${tone} ${
        emphasized ? 'page-history__summary-stat--emphasized' : ''
      }`.trim()}
    >
      <div className="page-history__summary-stat-value">{value}</div>
      <div className="page-history__summary-stat-label">{label}</div>
    </div>
  )
}

function DiffView({ comparison }: { comparison: RevisionComparison }) {
  const diff = useMemo(
    () => buildLineDiff(comparison.base.content, comparison.target.content),
    [comparison.base.content, comparison.target.content],
  )

  if (!comparison.contentChanged) {
    return (
      <div className="page-history__empty-message">
        No text difference between this revision and the active version.
      </div>
    )
  }

  return (
    <div className="page-history__diff">
      {diff.lines.map((line, index) => (
        <div
          key={`${line.kind}-${line.oldLineNumber}-${line.newLineNumber}-${index}`}
          className={`page-history__diff-line page-history__diff-line--${line.kind}`}
        >
          <span className="page-history__diff-gutter">
            {line.oldLineNumber ?? ''}
          </span>
          <span className="page-history__diff-gutter">
            {line.newLineNumber ?? ''}
          </span>
          <span className="page-history__diff-marker">
            {line.kind === 'added' ? '+' : line.kind === 'removed' ? '-' : ' '}
          </span>
          <code className="page-history__diff-content">
            {line.value || ' '}
          </code>
        </div>
      ))}
    </div>
  )
}

function ChangesPanel({ comparison }: { comparison: RevisionComparison }) {
  const diff = useMemo(
    () => buildLineDiff(comparison.base.content, comparison.target.content),
    [comparison.base.content, comparison.target.content],
  )

  const assetSummary = useMemo(() => {
    const counts = { added: 0, modified: 0, removed: 0 }
    comparison.assetChanges.forEach((change) => {
      counts[change.status] += 1
    })
    return counts
  }, [comparison.assetChanges])

  return (
    <div className="page-history__detail-stack">
      <section className="page-history__summary">
        <div className="page-history__section-heading">Change Summary</div>
        <div className="page-history__summary-grid">
          <SummaryStat
            label="Lines added since"
            value={String(diff.summary.addedLines)}
            emphasized={diff.summary.addedLines > 0}
            tone="added"
          />
          <SummaryStat
            label="Lines removed since"
            value={String(diff.summary.removedLines)}
            emphasized={diff.summary.removedLines > 0}
            tone="removed"
          />
          <SummaryStat
            label="Assets changed"
            value={String(comparison.assetChanges.length)}
            emphasized={comparison.assetChanges.length > 0}
          />
        </div>
      </section>

      <section className="page-history__section">
        <div className="page-history__section-heading">
          Diff{' '}
          <span className="page-history__section-heading-note">
            compared to the active version
          </span>
        </div>
        <DiffView comparison={comparison} />
      </section>

      {comparison.assetChanges.length > 0 ? (
        <details className="page-history__asset-details">
          <summary className="page-history__asset-summary">
            Assets ({comparison.assetChanges.length})
          </summary>
          <div className="page-history__asset-list">
            {comparison.assetChanges.map((change) => (
              <div
                key={`${change.name}-${change.status}`}
                className="page-history__asset-change"
              >
                <span className="page-history__asset-name">{change.name}</span>
                <span className="page-history__asset-meta">
                  {assetChangeLabel(change.status)}
                </span>
              </div>
            ))}
          </div>
          <div className="page-history__asset-summary-row">
            {assetSummary.added > 0 ? (
              <span>{assetSummary.added} added</span>
            ) : null}
            {assetSummary.modified > 0 ? (
              <span>{assetSummary.modified} replaced</span>
            ) : null}
            {assetSummary.removed > 0 ? (
              <span>{assetSummary.removed} removed</span>
            ) : null}
          </div>
        </details>
      ) : null}
    </div>
  )
}

function PreviewPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
  const pageId = snapshot.revision.pageId
  const revisionId = snapshot.revision.id

  const resolveAssetUrl = useCallback(
    (src: string) => {
      const normalizedSrc = src.startsWith('assets/') ? `/${src}` : src
      const assetPrefix = `/assets/${pageId}/`

      if (!normalizedSrc.startsWith(assetPrefix)) {
        return src
      }

      return buildRevisionAssetUrl(
        pageId,
        revisionId,
        normalizedSrc.slice(assetPrefix.length),
      )
    },
    [pageId, revisionId],
  )

  return (
    <div className="page-history__preview-panel custom-scrollbar">
      <div className="page-history__preview-body">
        <MarkdownPreview
          content={snapshot.content}
          path={snapshot.revision.path}
          resolveAssetUrl={resolveAssetUrl}
          enableHeadlineLinks={false}
        />
      </div>
    </div>
  )
}

function RawTextPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
  return (
    <div className="page-history__detail-stack">
      <section className="page-history__section">
        <div className="page-history__section-heading">Raw Text</div>
        <div className="custom-scrollbar markdown-code-block page-history__raw-text-block">
          <pre className="custom-scrollbar page-history__snapshot-content">
            <code>{snapshot.content || '(empty)'}</code>
          </pre>
        </div>
      </section>
    </div>
  )
}

function HistoryAssetItem({
  asset,
  pageId,
  revisionId,
}: {
  asset: RevisionSnapshot['assets'][number]
  pageId: string
  revisionId: string
}) {
  const assetUrl = withBasePath(
    buildRevisionAssetUrl(pageId, revisionId, asset.name),
  )
  const baseName = asset.name.split('/').pop() ?? asset.name

  return (
    <li className="group asset-item page-history__asset-item">
      <div className="flex min-w-0 flex-1 items-center gap-1">
        <AssetPreviewTooltip url={assetUrl} name={baseName}>
          {asset.mimeType?.startsWith('image/') ? (
            <img
              src={assetUrl}
              alt={baseName}
              className="asset-item__preview-image"
            />
          ) : (
            <div className="asset-item__preview-file">
              <FileText size={18} />
            </div>
          )}
        </AssetPreviewTooltip>

        <div className="page-history__asset-copy">
          <span className="asset-item__filename">{baseName}</span>
          <span className="page-history__asset-copy-meta">
            {asset.mimeType || 'application/octet-stream'} ·{' '}
            {Intl.NumberFormat().format(asset.sizeBytes)} bytes
          </span>
        </div>
      </div>

      <Button
        asChild
        variant="outline"
        size="icon"
        className="asset-item__action-button"
      >
        <a
          href={assetUrl}
          target="_blank"
          rel="noreferrer"
          title="Open asset"
          data-testid={`history-asset-open-${baseName}`}
        >
          <ExternalLink size={16} />
        </a>
      </Button>
      <Button
        asChild
        variant="outline"
        size="icon"
        className="asset-item__action-button"
      >
        <a
          href={assetUrl}
          download={baseName}
          title="Download asset"
          data-testid={`history-asset-download-${baseName}`}
        >
          <Download size={16} />
        </a>
      </Button>
    </li>
  )
}

function AssetsPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
  return (
    <div className="page-history__detail-stack">
      <section className="page-history__section">
        <div className="page-history__section-heading">Assets</div>
        {snapshot.assets.length === 0 ? (
          <div className="page-history__empty-message">
            No assets were stored with this revision.
          </div>
        ) : (
          <ul className="page-history__asset-list">
            {snapshot.assets.map((asset) => (
              <HistoryAssetItem
                key={`${asset.name}-${asset.sha256}`}
                asset={asset}
                pageId={snapshot.revision.pageId}
                revisionId={snapshot.revision.id}
              />
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}

export function PageHistoryContent({
  pageId,
  pageTitle,
  pageSlug,
  testidPrefix = 'page-history',
}: PageHistoryContentProps) {
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

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const groupedRevisions = useMemo(() => groupRevisions(revisions), [revisions])
  const isSelectedRevisionLatest =
    !!selectedRevision && selectedRevision.id === latestRevisionId

  const chips = useMemo(() => {
    if (!selectedRevision) return []

    const result = [
      `Revision slug: ${selectedRevision.slug || '/'}`,
      getPathLeaf(selectedRevision.path),
      revisionTriggerLabel(selectedRevision.type),
    ]

    if (pageSlug && pageSlug !== selectedRevision.slug) {
      result.unshift(`Current slug: ${pageSlug}`)
    }

    if (comparison) {
      result.push(`${comparison.assetChanges.length} asset changes`)
    } else if (snapshot) {
      result.push(`${snapshot.assets.length} Assets`)
    }

    return result
  }, [comparison, pageSlug, selectedRevision, snapshot])

  const structureChanges = useMemo(() => {
    if (!comparison) return []

    const changes: Array<{ label: string; from: string; to: string }> = []

    if (comparison.base.revision?.title !== comparison.target.revision?.title) {
      changes.push({
        label: 'Title',
        from: comparison.base.revision?.title || '(empty)',
        to: comparison.target.revision?.title || '(empty)',
      })
    }

    if (comparison.base.revision?.slug !== comparison.target.revision?.slug) {
      changes.push({
        label: 'Slug',
        from: comparison.base.revision?.slug || '(empty)',
        to: comparison.target.revision?.slug || '(empty)',
      })
    }

    return changes
  }, [comparison])

  // Preview is first and the default active tab so users immediately see the
  // rendered content of the selected revision without an extra click.
  const tabs: { id: HistoryTab; label: string }[] = [
    { id: 'preview', label: 'Preview' },
    { id: 'changes', label: 'Changes' },
    { id: 'raw', label: 'Raw Text' },
    { id: 'assets', label: 'Assets' },
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
      navigate(buildHistoryUrl(restoredPage.path), { replace: true })
      toast.success('Revision restored')
    } catch (err) {
      const mapped = mapApiError(err, 'Failed to restore revision')
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
        <div className="page-history__loading-state">Loading history...</div>
      )
    }

    if (listError) {
      return <ErrorNotice error={listError} />
    }

    if (!selectedRevision) {
      return (
        <EmptyState
          title="No revision selected"
          message="Select a revision from the list to view details."
        />
      )
    }

    if (detailLoading && !comparison && !snapshot) {
      return (
        <div className="page-history__loading-state">
          {activeTab === 'changes' || activeTab === 'assets'
            ? 'Loading diff...'
            : 'Loading preview...'}
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
          No preview available.
        </div>
      )
    }

    if (activeTab === 'changes') {
      if (isSelectedRevisionLatest) {
        return (
          <div className="page-history__empty-message page-history__empty-message--padded">
            No differences from the current version.
          </div>
        )
      }

      return comparison ? (
        <ChangesPanel comparison={comparison} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          No comparison data available.
        </div>
      )
    }

    if (activeTab === 'raw') {
      return snapshot ? (
        <RawTextPanel snapshot={snapshot} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          No raw text available.
        </div>
      )
    }

    return snapshot ? (
      <AssetsPanel snapshot={snapshot} />
    ) : (
      <div className="page-history__empty-message page-history__empty-message--padded">
        No asset data available.
      </div>
    )
  }

  const renderRevisionList = () => {
    if (listLoading) {
      return (
        <div className="page-history__list-status">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading history...
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
            ? 'No previous revisions yet. Older versions will appear here after more changes.'
            : 'No revisions yet. They will appear here after the page changes.'}
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
                        Active version
                      </RevisionBadge>
                    ) : null}
                  </div>
                  <div className="history-sidebar__item-meta">
                    {revisionMeta(revision)}
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
              {loadingMore ? 'Loading...' : 'Load more'}
            </Button>
          </div>
        ) : null}
      </>
    )
  }

  return (
    <div className="page-history" data-testid={`${testidPrefix}-content`}>
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
                  Revision History
                </div>
                {isMobile && selectedRevision ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setMobileListVisible(false)}
                  >
                    Show details
                  </Button>
                ) : null}
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
                aria-label="Resize revision list"
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
                  Revision by {displayAuthor(selectedRevision)} ·{' '}
                  {formatRelativeTime(selectedRevision.createdAt) ||
                    formatTimestamp(selectedRevision.createdAt)}
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
              {isMobile ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setMobileListVisible((current) => !current)}
                  data-testid={`${testidPrefix}-toggle-list`}
                >
                  <PanelLeftOpen className="h-4 w-4" />
                  Show revisions
                </Button>
              ) : null}
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
                  ? 'Restoring...'
                  : isSelectedRevisionLatest
                    ? 'Current version'
                    : 'Restore'}
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
