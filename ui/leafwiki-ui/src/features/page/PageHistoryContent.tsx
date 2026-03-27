import { Button } from '@/components/ui/button'
import { mapApiError, type ApiUiError } from '@/lib/api/errors'
import {
  compareRevisions,
  getLatestRevision,
  getRevisionSnapshot,
  listRevisions,
  type Revision,
  type RevisionAssetChange,
  type RevisionComparison,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { formatRelativeTime } from '@/lib/formatDate'
import { History, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

type HistoryMode = 'preview' | 'compare'

export type PageHistoryContentProps = {
  pageId: string
  pageTitle: string
  testidPrefix?: string
}

function revisionTypeLabel(type: string) {
  switch (type) {
    case 'content_update':
      return 'Content'
    case 'asset_update':
      return 'Assets'
    case 'structure_update':
      return 'Structure'
    case 'restore':
      return 'Restore'
    case 'delete':
      return 'Delete'
    default:
      return type
  }
}

function assetChangeLabel(status: RevisionAssetChange['status']) {
  switch (status) {
    case 'added':
      return 'Added'
    case 'removed':
      return 'Removed'
    case 'modified':
      return 'Modified'
    default:
      return status
  }
}

function displayAuthor(revision: Revision) {
  return revision.author?.username || revision.authorId || 'Unknown'
}

function ErrorNotice({ error }: { error: ApiUiError }) {
  return (
    <div className="page-history__error-notice">
      <div className="page-history__error-title">{error.message}</div>
      {error.detail ? (
        <div className="page-history__error-detail">{error.detail}</div>
      ) : null}
    </div>
  )
}

function SnapshotPanel({
  title,
  snapshot,
}: {
  title: string
  snapshot: RevisionSnapshot
}) {
  return (
    <div className="page-history__snapshot">
      <div className="page-history__snapshot-header">
        <div className="page-history__snapshot-title">{title}</div>
        <div className="page-history__snapshot-meta">
          {revisionTypeLabel(snapshot.revision.type)} by{' '}
          {displayAuthor(snapshot.revision)}
        </div>
      </div>

      <div className="page-history__snapshot-grid">
        <div className="page-history__snapshot-field">
          <div className="page-history__snapshot-label">Path</div>
          <div className="page-history__snapshot-value">
            {snapshot.revision.path}
          </div>
        </div>
        <div className="page-history__snapshot-field">
          <div className="page-history__snapshot-label">Timestamp</div>
          <div className="page-history__snapshot-value">
            {snapshot.revision.createdAt}
          </div>
        </div>
      </div>

      <div>
        <div className="page-history__section-title">Content</div>
        <pre className="page-history__snapshot-content">
          {snapshot.content || '(empty)'}
        </pre>
      </div>

      <div>
        <div className="page-history__section-title">Assets</div>
        {snapshot.assets.length === 0 ? (
          <div className="page-history__empty-message">
            No assets stored for this revision.
          </div>
        ) : (
          <div className="page-history__asset-list">
            {snapshot.assets.map((asset) => (
              <div
                key={`${asset.name}-${asset.sha256}`}
                className="page-history__asset-item"
              >
                <div className="page-history__asset-name">{asset.name}</div>
                <div className="page-history__asset-meta">
                  {asset.mimeType || 'unknown'} · {asset.sizeBytes} bytes
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export function PageHistoryContent({
  pageId,
  pageTitle,
  testidPrefix = 'page-history',
}: PageHistoryContentProps) {
  const [revisions, setRevisions] = useState<Revision[]>([])
  const [selectedRevisionId, setSelectedRevisionId] = useState<string | null>(
    null,
  )
  const [latestRevisionId, setLatestRevisionId] = useState<string | null>(null)
  const [snapshot, setSnapshot] = useState<RevisionSnapshot | null>(null)
  const [comparison, setComparison] = useState<RevisionComparison | null>(null)
  const [mode, setMode] = useState<HistoryMode>('preview')
  const [listLoading, setListLoading] = useState(false)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [compareLoading, setCompareLoading] = useState(false)
  const [listError, setListError] = useState<ApiUiError | null>(null)
  const [previewError, setPreviewError] = useState<ApiUiError | null>(null)
  const [nextCursor, setNextCursor] = useState('')
  const [loadingMore, setLoadingMore] = useState(false)

  useEffect(() => {
    if (!pageId) return

    let cancelled = false

    const load = async () => {
      setListLoading(true)
      setListError(null)
      setPreviewError(null)
      setSnapshot(null)
      setComparison(null)
      setSelectedRevisionId(null)
      setLatestRevisionId(null)
      setMode('preview')
      try {
        const [historyData, latestRevision] = await Promise.all([
          listRevisions(pageId),
          getLatestRevision(pageId),
        ])
        if (cancelled) return
        setRevisions(historyData.revisions)
        setNextCursor(historyData.nextCursor)
        setLatestRevisionId(latestRevision.id)
        const firstRevision = historyData.revisions[0]
        if (firstRevision) {
          setSelectedRevisionId(firstRevision.id)
        }
      } catch (err) {
        if (cancelled) return
        setListError(mapApiError(err, 'Failed to load page history'))
        setRevisions([])
        setNextCursor('')
        setLatestRevisionId(null)
      } finally {
        if (!cancelled) {
          setListLoading(false)
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [pageId])

  useEffect(() => {
    if (!pageId || !selectedRevisionId || mode !== 'preview') return

    let cancelled = false

    const loadSnapshot = async () => {
      setPreviewLoading(true)
      setPreviewError(null)
      try {
        const data = await getRevisionSnapshot(pageId, selectedRevisionId)
        if (cancelled) return
        setSnapshot(data)
      } catch (err) {
        if (cancelled) return
        setSnapshot(null)
        setPreviewError(mapApiError(err, 'Failed to load revision preview'))
      } finally {
        if (!cancelled) {
          setPreviewLoading(false)
        }
      }
    }

    void loadSnapshot()

    return () => {
      cancelled = true
    }
  }, [mode, pageId, selectedRevisionId])

  useEffect(() => {
    if (
      !pageId ||
      !selectedRevisionId ||
      !latestRevisionId ||
      mode !== 'compare'
    ) {
      return
    }

    let cancelled = false

    const loadComparison = async () => {
      setCompareLoading(true)
      setPreviewError(null)
      try {
        const data = await compareRevisions(
          pageId,
          selectedRevisionId,
          latestRevisionId,
        )
        if (cancelled) return
        setComparison(data)
      } catch (err) {
        if (cancelled) return
        setComparison(null)
        setPreviewError(mapApiError(err, 'Failed to compare revisions'))
      } finally {
        if (!cancelled) {
          setCompareLoading(false)
        }
      }
    }

    void loadComparison()

    return () => {
      cancelled = true
    }
  }, [latestRevisionId, mode, pageId, selectedRevisionId])

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const isComparingCurrent =
    mode === 'compare' &&
    !!selectedRevisionId &&
    selectedRevisionId === latestRevisionId

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return
    setLoadingMore(true)
    setListError(null)
    try {
      const data = await listRevisions(pageId, nextCursor)
      setRevisions((current) => [...current, ...data.revisions])
      setNextCursor(data.nextCursor)
    } catch (err) {
      setListError(mapApiError(err, 'Failed to load more revisions'))
    } finally {
      setLoadingMore(false)
    }
  }

  const handleSelectRevision = (revisionId: string) => {
    setSelectedRevisionId(revisionId)
    setPreviewError(null)
    setSnapshot(null)
    setComparison(null)
  }

  return (
    <div className="page-history" data-testid={`${testidPrefix}-content`}>
      <div className="page-history__toolbar">
        <div className="page-history__toolbar-title-group">
          <div className="page-history__toolbar-label">History</div>
          <div className="page-history__toolbar-title">{pageTitle}</div>
        </div>
        <div className="page-history__mode-switch">
          <Button
            variant={mode === 'preview' ? 'default' : 'outline'}
            onClick={() => setMode('preview')}
            disabled={!selectedRevisionId}
            data-testid={`${testidPrefix}-preview-mode`}
          >
            Preview
          </Button>
          <Button
            variant={mode === 'compare' ? 'default' : 'outline'}
            onClick={() => setMode('compare')}
            disabled={!selectedRevisionId || !latestRevisionId}
            data-testid={`${testidPrefix}-compare-mode`}
          >
            Compare to current
          </Button>
        </div>
      </div>

      <div className="page-history__layout">
        <div className="page-history__panel">
          <div className="page-history__panel-header">
            <div className="page-history__panel-title">
              <History className="h-4 w-4" />
              Revisions
            </div>
          </div>
          <div className="page-history__revision-list custom-scrollbar">
            {listLoading ? (
              <div className="page-history__loading-state">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading history...
              </div>
            ) : listError ? (
              <ErrorNotice error={listError} />
            ) : revisions.length === 0 ? (
              <div className="page-history__empty-message page-history__empty-message--padded">
                No revisions available yet.
              </div>
            ) : (
              <div className="page-history__revision-items">
                {revisions.map((revision) => {
                  const selected = revision.id === selectedRevisionId
                  return (
                    <button
                      key={revision.id}
                      type="button"
                      className={`page-history__revision-button ${
                        selected ? 'page-history__revision-button--active' : ''
                      }`}
                      onClick={() => handleSelectRevision(revision.id)}
                      data-testid={`${testidPrefix}-revision-${revision.id}`}
                    >
                      <div className="page-history__revision-row">
                        <span className="page-history__revision-type">
                          {revisionTypeLabel(revision.type)}
                        </span>
                        <span className="page-history__revision-time">
                          {formatRelativeTime(revision.createdAt) ||
                            revision.createdAt}
                        </span>
                      </div>
                      <div className="page-history__revision-author">
                        {displayAuthor(revision)}
                      </div>
                      {revision.summary ? (
                        <div className="page-history__revision-summary">
                          {revision.summary}
                        </div>
                      ) : null}
                    </button>
                  )
                })}
                {nextCursor ? (
                  <Button
                    variant="outline"
                    className="w-full"
                    onClick={loadMore}
                    disabled={loadingMore}
                    data-testid={`${testidPrefix}-load-more`}
                  >
                    {loadingMore ? 'Loading...' : 'Load more'}
                  </Button>
                ) : null}
              </div>
            )}
          </div>
        </div>

        <div className="page-history__panel page-history__panel--detail">
          <div className="page-history__panel-header page-history__panel-header--detail">
            <div>
              <div className="page-history__panel-heading">
                {mode === 'preview' ? 'Revision Preview' : 'Compare to Current'}
              </div>
              {selectedRevision ? (
                <div className="page-history__panel-subtitle">
                  {revisionTypeLabel(selectedRevision.type)} by{' '}
                  {displayAuthor(selectedRevision)}
                </div>
              ) : null}
            </div>
          </div>

          <div className="page-history__detail-content custom-scrollbar">
            {mode === 'preview' ? (
              previewLoading ? (
                <div className="page-history__loading-state">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading preview...
                </div>
              ) : previewError ? (
                <ErrorNotice error={previewError} />
              ) : snapshot ? (
                <SnapshotPanel title="Selected revision" snapshot={snapshot} />
              ) : (
                <div className="page-history__empty-message">
                  Select a revision to preview it.
                </div>
              )
            ) : compareLoading ? (
              <div className="page-history__loading-state">
                <Loader2 className="h-4 w-4 animate-spin" />
                Comparing with current version...
              </div>
            ) : previewError ? (
              <ErrorNotice error={previewError} />
            ) : comparison ? (
              <div className="page-history__comparison">
                <div className="page-history__comparison-notice">
                  {isComparingCurrent
                    ? 'You selected the current revision. There are no content or asset changes to compare.'
                    : comparison.contentChanged
                      ? 'Content changed between the selected revision and the current version.'
                      : 'Content is unchanged between the selected revision and the current version.'}
                </div>

                <div>
                  <div className="page-history__section-title">
                    Asset changes
                  </div>
                  {comparison.assetChanges.length === 0 ? (
                    <div className="page-history__empty-message">
                      No asset changes between the selected revision and the
                      current version.
                    </div>
                  ) : (
                    <div className="page-history__asset-list">
                      {comparison.assetChanges.map((change) => (
                        <div
                          key={`${change.name}-${change.status}`}
                          className="page-history__asset-change"
                        >
                          <span className="page-history__asset-name">
                            {change.name}
                          </span>
                          <span className="page-history__asset-meta">
                            {assetChangeLabel(change.status)}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="page-history__comparison-grid">
                  <SnapshotPanel
                    title="Selected revision"
                    snapshot={comparison.base}
                  />
                  <SnapshotPanel
                    title="Current version"
                    snapshot={comparison.target}
                  />
                </div>
              </div>
            ) : (
              <div className="page-history__empty-message">
                Select a revision to compare it with the current version.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
