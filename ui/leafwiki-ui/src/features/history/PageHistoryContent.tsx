import { Button } from '@/components/ui/button'
import { type ApiUiError } from '@/lib/api/errors'
import {
  buildRevisionAssetUrl,
  type Revision,
  type RevisionAssetChange,
  type RevisionComparison,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { formatRelativeTime } from '@/lib/formatDate'
import { type ReactNode, useMemo } from 'react'
import MarkdownPreview from '../preview/MarkdownPreview'
import { type HistoryTab, usePageHistoryStore } from './pageHistory'

export type PageHistoryContentProps = {
  pageId: string
  pageTitle: string
  testidPrefix?: string
}

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
    summary: {
      addedLines,
      removedLines,
    },
  }
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

function MetaChip({ children }: { children: ReactNode }) {
  return <span className="page-history__meta-chip">{children}</span>
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
}: {
  label: string
  value: string
  emphasized?: boolean
}) {
  return (
    <div
      className={`page-history__summary-stat ${
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
        No text difference between this revision and the current version.
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
    const counts = {
      added: 0,
      modified: 0,
      removed: 0,
    }

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
            label="Lines added"
            value={String(diff.summary.addedLines)}
            emphasized={diff.summary.addedLines > 0}
          />
          <SummaryStat
            label="Lines removed"
            value={String(diff.summary.removedLines)}
            emphasized={diff.summary.removedLines > 0}
          />
          <SummaryStat
            label="Assets changed"
            value={String(comparison.assetChanges.length)}
            emphasized={comparison.assetChanges.length > 0}
          />
          <SummaryStat
            label="Revision type"
            value={revisionTypeLabel(comparison.base.revision.type)}
          />
        </div>
      </section>

      <section className="page-history__section">
        <div className="page-history__section-heading">Diff</div>
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
  const resolveAssetUrl = (src: string) => {
    const normalizedSrc = src.startsWith('assets/') ? `/${src}` : src
    const assetPrefix = `/assets/${snapshot.revision.pageId}/`

    if (!normalizedSrc.startsWith(assetPrefix)) {
      return src
    }

    return buildRevisionAssetUrl(
      snapshot.revision.pageId,
      snapshot.revision.id,
      normalizedSrc.slice(assetPrefix.length),
    )
  }

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
        <pre className="page-history__snapshot-content">
          {snapshot.content || '(empty)'}
        </pre>
      </section>
    </div>
  )
}

function AssetsPanel({ comparison }: { comparison: RevisionComparison }) {
  return (
    <div className="page-history__detail-stack">
      <section className="page-history__section">
        <div className="page-history__section-heading">Asset Changes</div>
        {comparison.assetChanges.length === 0 ? (
          <div className="page-history__empty-message">
            No asset changes between this revision and the current version.
          </div>
        ) : (
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
        )}
      </section>
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
  const snapshot = usePageHistoryStore((state) => state.snapshot)
  const comparison = usePageHistoryStore((state) => state.comparison)
  const activeTab = usePageHistoryStore((state) => state.activeTab)
  const listLoading = usePageHistoryStore((state) => state.listLoading)
  const previewLoading = usePageHistoryStore((state) => state.previewLoading)
  const compareLoading = usePageHistoryStore((state) => state.compareLoading)
  const listError = usePageHistoryStore((state) => state.listError)
  const previewError = usePageHistoryStore((state) => state.previewError)
  const setActiveTab = usePageHistoryStore((state) => state.setActiveTab)

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const chips = useMemo(() => {
    if (!selectedRevision) return []

    const result = [
      selectedRevision.path,
      revisionTypeLabel(selectedRevision.type),
    ]

    if (comparison) {
      result.push(`${comparison.assetChanges.length} asset changes`)
    } else if (snapshot) {
      result.push(`${snapshot.assets.length} Assets`)
    }

    return result
  }, [comparison, selectedRevision, snapshot])

  const tabs: { id: HistoryTab; label: string }[] = [
    { id: 'changes', label: 'Changes' },
    { id: 'preview', label: 'Preview' },
    { id: 'raw', label: 'Raw Text' },
    { id: 'assets', label: 'Assets' },
  ]

  const detailLoading =
    activeTab === 'changes' || activeTab === 'assets'
      ? compareLoading
      : previewLoading

  if (!listLoading && !listError && revisions.length === 0) {
    return (
      <div className="page-history" data-testid={`${testidPrefix}-content`}>
        <div className="page-history__header">
          <div className="page-history__header-copy">
            <div className="page-history__header-title">{pageTitle}</div>
            <div className="page-history__header-subtitle">
              Revisions will appear here after this page is changed.
            </div>
          </div>
        </div>

        <div className="page-history__workspace">
          <div className="page-history__panel page-history__panel--detail">
            <div className="page-history__detail-content custom-scrollbar">
              <EmptyState
                title="No revisions yet"
                message="This page does not have any saved revision history yet."
              />
            </div>
          </div>
        </div>
      </div>
    )
  }

  const renderPanelContent = () => {
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
          title="No revision to display"
          message="There is currently no revision available that can be shown for this page."
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

    if (activeTab === 'changes') {
      return comparison ? (
        <ChangesPanel comparison={comparison} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          No comparison data available.
        </div>
      )
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

    if (activeTab === 'raw') {
      return snapshot ? (
        <RawTextPanel snapshot={snapshot} />
      ) : (
        <div className="page-history__empty-message page-history__empty-message--padded">
          No raw text available.
        </div>
      )
    }

    return comparison ? (
      <AssetsPanel comparison={comparison} />
    ) : (
      <div className="page-history__empty-message page-history__empty-message--padded">
        No asset data available.
      </div>
    )
  }

  return (
    <div className="page-history" data-testid={`${testidPrefix}-content`}>
      <div className="page-history__header">
        <div className="page-history__header-copy">
          <div className="page-history__header-title">{pageTitle}</div>
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
        </div>

        <div className="page-history__actions">
          <Button
            variant="default"
            disabled
            data-testid={`${testidPrefix}-restore`}
          >
            Restore
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

      <div className="page-history__workspace">
        <div className="page-history__panel page-history__panel--detail">
          <div className="page-history__detail-content custom-scrollbar">
            {renderPanelContent()}
          </div>
        </div>
      </div>
    </div>
  )
}
