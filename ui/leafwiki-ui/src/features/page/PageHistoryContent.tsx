import { Button } from '@/components/ui/button'
import { type ApiUiError } from '@/lib/api/errors'
import {
  type Revision,
  type RevisionAssetChange,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { Loader2 } from 'lucide-react'
import { useMemo } from 'react'
import { usePageHistoryStore } from './pageHistory'

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
  pageTitle,
  testidPrefix = 'page-history',
}: PageHistoryContentProps) {
  const revisions = usePageHistoryStore((state) => state.revisions)
  const selectedRevisionId = usePageHistoryStore(
    (state) => state.selectedRevisionId,
  )
  const latestRevisionId = usePageHistoryStore(
    (state) => state.latestRevisionId,
  )
  const snapshot = usePageHistoryStore((state) => state.snapshot)
  const comparison = usePageHistoryStore((state) => state.comparison)
  const mode = usePageHistoryStore((state) => state.mode)
  const listLoading = usePageHistoryStore((state) => state.listLoading)
  const previewLoading = usePageHistoryStore((state) => state.previewLoading)
  const compareLoading = usePageHistoryStore((state) => state.compareLoading)
  const listError = usePageHistoryStore((state) => state.listError)
  const previewError = usePageHistoryStore((state) => state.previewError)
  const setMode = usePageHistoryStore((state) => state.setMode)

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const isComparingCurrent =
    mode === 'compare' &&
    !!selectedRevisionId &&
    selectedRevisionId === latestRevisionId

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

      <div className="page-history__workspace">
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
            ) : mode === 'preview' ? (
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
                <div className="page-history__empty-message page-history__empty-message--padded">
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

                <div className="page-history__comparison-section">
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
              <div className="page-history__empty-message page-history__empty-message--padded">
                Select a revision to compare it with the current version.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
