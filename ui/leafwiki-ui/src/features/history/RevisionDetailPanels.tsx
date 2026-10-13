import { Button } from '@/components/ui/button'
import {
  buildRevisionAssetUrl,
  type RevisionAssetChange,
  type RevisionComparison,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { withBasePath } from '@/lib/routePath'
import { Download, ExternalLink, FileText } from 'lucide-react'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { AssetPreviewTooltip } from '../assets/AssetPreviewTooltip'
import MarkdownPreview from '../preview/MarkdownPreview'
import { buildLineDiff } from './lineDiff'

// Detail panels for a selected revision: changes (summary + line diff),
// rendered preview, raw text and assets.

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

function DiffView({
  comparison,
  diff,
}: {
  comparison: RevisionComparison
  diff: ReturnType<typeof buildLineDiff>
}) {
  const { t } = useTranslation('history')

  if (!comparison.contentChanged) {
    return (
      <div className="page-history__empty-message">
        {t('diff.noTextDifference')}
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

export function ChangesPanel({
  comparison,
}: {
  comparison: RevisionComparison
}) {
  const { t } = useTranslation('history')

  const assetChangeLabel = (status: RevisionAssetChange['status']) => {
    switch (status) {
      case 'added':
        return t('assetChange.added')
      case 'removed':
        return t('assetChange.removed')
      case 'modified':
        return t('assetChange.replaced')
      default:
        return status
    }
  }

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
        <div className="page-history__section-heading">
          {t('changes.summaryHeading')}
        </div>
        <div className="page-history__summary-grid">
          <SummaryStat
            label={t('changes.linesAdded')}
            value={String(diff.summary.addedLines)}
            emphasized={diff.summary.addedLines > 0}
            tone="added"
          />
          <SummaryStat
            label={t('changes.linesRemoved')}
            value={String(diff.summary.removedLines)}
            emphasized={diff.summary.removedLines > 0}
            tone="removed"
          />
          <SummaryStat
            label={t('changes.assetsChanged')}
            value={String(comparison.assetChanges.length)}
            emphasized={comparison.assetChanges.length > 0}
          />
        </div>
      </section>

      <section className="page-history__section">
        <div className="page-history__section-heading">
          {t('changes.diffHeading')}{' '}
          <span className="page-history__section-heading-note">
            {t('changes.diffNote')}
          </span>
        </div>
        <DiffView comparison={comparison} diff={diff} />
      </section>

      {comparison.assetChanges.length > 0 ? (
        <details className="page-history__asset-details">
          <summary className="page-history__asset-summary">
            {t('changes.assetsDetails', {
              count: comparison.assetChanges.length,
            })}
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
              <span>
                {t('changes.assetAdded', { count: assetSummary.added })}
              </span>
            ) : null}
            {assetSummary.modified > 0 ? (
              <span>
                {t('changes.assetReplaced', { count: assetSummary.modified })}
              </span>
            ) : null}
            {assetSummary.removed > 0 ? (
              <span>
                {t('changes.assetRemoved', { count: assetSummary.removed })}
              </span>
            ) : null}
          </div>
        </details>
      ) : null}
    </div>
  )
}

export function PreviewPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
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

export function RawTextPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
  const { t } = useTranslation('history')
  return (
    <div className="page-history__detail-stack">
      <section className="page-history__section">
        <div className="page-history__section-heading">
          {t('rawText.heading')}
        </div>
        <div className="custom-scrollbar markdown-code-block page-history__raw-text-block">
          <pre className="custom-scrollbar page-history__snapshot-content">
            <code>{snapshot.content || t('rawText.empty')}</code>
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
  const { t } = useTranslation('history')
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
            {asset.mimeType || t('assetsPanel.octetStream')} ·{' '}
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
          title={t('assetsPanel.openAsset')}
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
          title={t('assetsPanel.downloadAsset')}
          data-testid={`history-asset-download-${baseName}`}
        >
          <Download size={16} />
        </a>
      </Button>
    </li>
  )
}

export function AssetsPanel({ snapshot }: { snapshot: RevisionSnapshot }) {
  const { t } = useTranslation('history')
  return (
    <div className="page-history__detail-stack">
      <section className="page-history__section">
        <div className="page-history__section-heading">
          {t('assetsPanel.heading')}
        </div>
        {snapshot.assets.length === 0 ? (
          <div className="page-history__empty-message">
            {t('assetsPanel.empty')}
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
