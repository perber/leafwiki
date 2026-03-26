import BaseDialog from '@/components/BaseDialog'
import { Button } from '@/components/ui/button'
import { asApiLocalizedError, getErrorMessage } from '@/lib/api/errors'
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
import { DIALOG_PAGE_HISTORY } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { History, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

export type PageHistoryDialogProps = {
  pageId: string
  pageTitle: string
}

type HistoryMode = 'preview' | 'compare'

type LocalizedUiError = {
  message: string
  code?: string
  template?: string
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

function toUiError(err: unknown, fallback: string): LocalizedUiError {
  const localized = asApiLocalizedError(err)
  if (localized) {
    return {
      message: localized.message,
      code: localized.code,
      template: localized.template,
    }
  }

  return {
    message: getErrorMessage(err, fallback),
  }
}

function ErrorNotice({ error }: { error: LocalizedUiError }) {
  return (
    <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
      <div>{error.message}</div>
      {error.code || error.template ? (
        <div className="mt-1 text-xs text-red-600/90">
          {error.code ? `Code: ${error.code}` : null}
          {error.code && error.template ? ' · ' : null}
          {error.template ? `Template: ${error.template}` : null}
        </div>
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
    <div className="space-y-4 rounded-md border p-4">
      <div>
        <div className="text-sm font-medium">{title}</div>
        <div className="mt-1 text-xs text-muted-foreground">
          {revisionTypeLabel(snapshot.revision.type)} by {displayAuthor(snapshot.revision)}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-md bg-slate-50 p-3">
          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Path</div>
          <div className="mt-1 text-sm">{snapshot.revision.path}</div>
        </div>
        <div className="rounded-md bg-slate-50 p-3">
          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Timestamp</div>
          <div className="mt-1 text-sm">{snapshot.revision.createdAt}</div>
        </div>
      </div>

      <div>
        <div className="mb-2 text-sm font-medium">Content</div>
        <pre className="max-h-[28vh] overflow-auto rounded-md border bg-slate-50 p-4 text-sm whitespace-pre-wrap break-words">
          {snapshot.content || '(empty)'}
        </pre>
      </div>

      <div>
        <div className="mb-2 text-sm font-medium">Assets</div>
        {snapshot.assets.length === 0 ? (
          <div className="text-sm text-muted-foreground">No assets stored for this revision.</div>
        ) : (
          <div className="space-y-2">
            {snapshot.assets.map((asset) => (
              <div
                key={`${asset.name}-${asset.sha256}`}
                className="rounded-md border px-3 py-2 text-sm"
              >
                <div className="font-medium">{asset.name}</div>
                <div className="text-xs text-muted-foreground">
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

export function PageHistoryDialog({ pageId, pageTitle }: PageHistoryDialogProps) {
  const open = useDialogsStore((s) => s.dialogType === DIALOG_PAGE_HISTORY)
  const [revisions, setRevisions] = useState<Revision[]>([])
  const [selectedRevisionId, setSelectedRevisionId] = useState<string | null>(null)
  const [latestRevisionId, setLatestRevisionId] = useState<string | null>(null)
  const [snapshot, setSnapshot] = useState<RevisionSnapshot | null>(null)
  const [comparison, setComparison] = useState<RevisionComparison | null>(null)
  const [mode, setMode] = useState<HistoryMode>('preview')
  const [listLoading, setListLoading] = useState(false)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [compareLoading, setCompareLoading] = useState(false)
  const [listError, setListError] = useState<LocalizedUiError | null>(null)
  const [previewError, setPreviewError] = useState<LocalizedUiError | null>(null)
  const [nextCursor, setNextCursor] = useState('')
  const [loadingMore, setLoadingMore] = useState(false)

  useEffect(() => {
    if (!open || !pageId) return

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
        setListError(toUiError(err, 'Failed to load page history'))
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
  }, [open, pageId])

  useEffect(() => {
    if (!open || !pageId || !selectedRevisionId || mode !== 'preview') return

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
        setPreviewError(toUiError(err, 'Failed to load revision preview'))
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
  }, [mode, open, pageId, selectedRevisionId])

  useEffect(() => {
    if (
      !open ||
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
        const data = await compareRevisions(pageId, selectedRevisionId, latestRevisionId)
        if (cancelled) return
        setComparison(data)
      } catch (err) {
        if (cancelled) return
        setComparison(null)
        setPreviewError(toUiError(err, 'Failed to compare revisions'))
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
  }, [latestRevisionId, mode, open, pageId, selectedRevisionId])

  const selectedRevision = useMemo(
    () => revisions.find((item) => item.id === selectedRevisionId) ?? null,
    [revisions, selectedRevisionId],
  )

  const isComparingCurrent =
    mode === 'compare' && !!selectedRevisionId && selectedRevisionId === latestRevisionId

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return
    setLoadingMore(true)
    setListError(null)
    try {
      const data = await listRevisions(pageId, nextCursor)
      setRevisions((current) => [...current, ...data.revisions])
      setNextCursor(data.nextCursor)
    } catch (err) {
      setListError(toUiError(err, 'Failed to load more revisions'))
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
    <BaseDialog
      dialogType={DIALOG_PAGE_HISTORY}
      dialogTitle={`History for ${pageTitle}`}
      dialogDescription="Browse previous revisions and preview historical content for this page."
      onClose={() => true}
      onConfirm={async () => true}
      defaultAction="cancel"
      testidPrefix="page-history-dialog"
      contentClassName="max-w-7xl"
      cancelButton={{
        label: 'Close',
        variant: 'outline',
        autoFocus: true,
      }}
    >
      <div className="grid gap-4 md:grid-cols-[280px_minmax(0,1fr)]">
        <div className="rounded-md border">
          <div className="border-b px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <History className="h-4 w-4" />
              Revisions
            </div>
          </div>
          <div className="max-h-[60vh] overflow-y-auto p-2">
            {listLoading ? (
              <div className="flex items-center gap-2 px-2 py-6 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading history...
              </div>
            ) : listError ? (
              <ErrorNotice error={listError} />
            ) : revisions.length === 0 ? (
              <div className="px-2 py-6 text-sm text-muted-foreground">
                No revisions available yet.
              </div>
            ) : (
              <div className="space-y-2">
                {revisions.map((revision) => {
                  const selected = revision.id === selectedRevisionId
                  return (
                    <button
                      key={revision.id}
                      type="button"
                      className={`w-full rounded-md border px-3 py-3 text-left transition ${
                        selected
                          ? 'border-blue-300 bg-blue-50'
                          : 'border-transparent hover:border-slate-200 hover:bg-slate-50'
                      }`}
                      onClick={() => handleSelectRevision(revision.id)}
                      data-testid={`page-history-dialog-revision-${revision.id}`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-sm font-medium">{revisionTypeLabel(revision.type)}</span>
                        <span className="text-xs text-muted-foreground">
                          {formatRelativeTime(revision.createdAt) || revision.createdAt}
                        </span>
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {displayAuthor(revision)}
                      </div>
                      {revision.summary ? (
                        <div className="mt-2 text-sm text-foreground/90">{revision.summary}</div>
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
                    data-testid="page-history-dialog-load-more"
                  >
                    {loadingMore ? 'Loading...' : 'Load more'}
                  </Button>
                ) : null}
              </div>
            )}
          </div>
        </div>

        <div className="min-h-[60vh] rounded-md border">
          <div className="flex flex-wrap items-start justify-between gap-3 border-b px-4 py-3">
            <div>
              <div className="text-sm font-medium">
                {mode === 'preview' ? 'Revision Preview' : 'Compare to Current'}
              </div>
              {selectedRevision ? (
                <div className="mt-1 text-xs text-muted-foreground">
                  {revisionTypeLabel(selectedRevision.type)} by {displayAuthor(selectedRevision)}
                </div>
              ) : null}
            </div>
            <div className="flex gap-2">
              <Button
                variant={mode === 'preview' ? 'default' : 'outline'}
                onClick={() => setMode('preview')}
                disabled={!selectedRevisionId}
                data-testid="page-history-dialog-preview-mode"
              >
                Preview
              </Button>
              <Button
                variant={mode === 'compare' ? 'default' : 'outline'}
                onClick={() => setMode('compare')}
                disabled={!selectedRevisionId || !latestRevisionId}
                data-testid="page-history-dialog-compare-mode"
              >
                Compare to current
              </Button>
            </div>
          </div>

          <div className="max-h-[60vh] space-y-4 overflow-y-auto p-4">
            {mode === 'preview' ? (
              previewLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading preview...
                </div>
              ) : previewError ? (
                <ErrorNotice error={previewError} />
              ) : snapshot ? (
                <SnapshotPanel title="Selected revision" snapshot={snapshot} />
              ) : (
                <div className="text-sm text-muted-foreground">
                  Select a revision to preview it.
                </div>
              )
            ) : compareLoading ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Comparing with current version...
              </div>
            ) : previewError ? (
              <ErrorNotice error={previewError} />
            ) : comparison ? (
              <div className="space-y-4">
                <div className="rounded-md border bg-slate-50 px-3 py-2 text-sm">
                  {isComparingCurrent
                    ? 'You selected the current revision. There are no content or asset changes to compare.'
                    : comparison.contentChanged
                      ? 'Content changed between the selected revision and the current version.'
                      : 'Content is unchanged between the selected revision and the current version.'}
                </div>

                <div>
                  <div className="mb-2 text-sm font-medium">Asset changes</div>
                  {comparison.assetChanges.length === 0 ? (
                    <div className="text-sm text-muted-foreground">
                      No asset changes between the selected revision and the current version.
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {comparison.assetChanges.map((change) => (
                        <div
                          key={`${change.name}-${change.status}`}
                          className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
                        >
                          <span className="font-medium">{change.name}</span>
                          <span className="text-xs text-muted-foreground">
                            {assetChangeLabel(change.status)}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <SnapshotPanel title="Selected revision" snapshot={comparison.base} />
                  <SnapshotPanel title="Current version" snapshot={comparison.target} />
                </div>
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">
                Select a revision to compare it with the current version.
              </div>
            )}
          </div>
        </div>
      </div>
    </BaseDialog>
  )
}
