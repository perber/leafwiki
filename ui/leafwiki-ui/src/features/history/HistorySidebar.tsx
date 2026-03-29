import {
  ListView,
  ListViewItem,
  ListViewList,
  ListViewStatus,
} from '@/components/ListView'
import { Button } from '@/components/ui/button'
import { type Revision } from '@/lib/api/revisions'
import { formatRelativeTime } from '@/lib/formatDate'
import { Loader2 } from 'lucide-react'
import { useMemo } from 'react'
import { loadMorePageHistory, usePageHistoryStore } from './pageHistory'

type RevisionGroup = {
  label: string
  revisions: Revision[]
}

function displayAuthor(username?: string, authorId?: string) {
  return username || authorId || 'Unknown'
}

function groupLabel(value?: string) {
  if (!value) return 'Unknown'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'

  const today = new Date()
  const startOfToday = new Date(
    today.getFullYear(),
    today.getMonth(),
    today.getDate(),
  )
  const startOfDate = new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  )

  const diffDays = Math.round(
    (startOfToday.getTime() - startOfDate.getTime()) / (1000 * 60 * 60 * 24),
  )

  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'

  return new Intl.DateTimeFormat(undefined, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(date)
}

function revisionTitle(revision: Revision) {
  if (revision.summary?.trim()) return revision.summary.trim()

  switch (revision.type) {
    case 'content_update':
      return 'Content changed'
    case 'asset_update':
      return 'Assets changed'
    case 'structure_update':
      return 'Structure updated'
    case 'restore':
      return 'Revision restored'
    case 'delete':
      return 'Page deleted'
    default:
      return revision.type
  }
}

function revisionMeta(revision: Revision) {
  const author = displayAuthor(revision.author?.username, revision.authorId)
  const time = formatRelativeTime(revision.createdAt) || revision.createdAt

  return `${author} · ${time}`
}

function groupRevisions(revisions: Revision[]): RevisionGroup[] {
  const groups: RevisionGroup[] = []

  revisions.forEach((revision) => {
    const label = groupLabel(revision.createdAt)
    const existingGroup = groups[groups.length - 1]

    if (!existingGroup || existingGroup.label !== label) {
      groups.push({
        label,
        revisions: [revision],
      })
      return
    }

    existingGroup.revisions.push(revision)
  })

  return groups
}

export function HistorySidebar() {
  const revisions = usePageHistoryStore((state) => state.revisions)
  const selectedRevisionId = usePageHistoryStore(
    (state) => state.selectedRevisionId,
  )
  const listLoading = usePageHistoryStore((state) => state.listLoading)
  const listError = usePageHistoryStore((state) => state.listError)
  const nextCursor = usePageHistoryStore((state) => state.nextCursor)
  const loadingMore = usePageHistoryStore((state) => state.loadingMore)
  const selectRevision = usePageHistoryStore((state) => state.selectRevision)

  const groupedRevisions = useMemo(() => groupRevisions(revisions), [revisions])

  return (
    <ListView
      as="div"
      className="history-sidebar"
      contentClassName="history-sidebar__content"
      testId="history-sidebar"
    >
      {listLoading ? (
        <ListViewStatus>
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading history...
        </ListViewStatus>
      ) : listError ? (
        <ListViewStatus error>{listError.message}</ListViewStatus>
      ) : revisions.length === 0 ? (
        <ListViewStatus>No revisions available yet.</ListViewStatus>
      ) : (
        <ListViewList>
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
                    onClick={() => selectRevision(revision.id)}
                    testId={`history-sidebar-revision-${revision.id}`}
                  >
                    <div className="history-sidebar__item-title">
                      {revisionTitle(revision)}
                    </div>
                    <div className="history-sidebar__item-meta">
                      {revisionMeta(revision)}
                    </div>
                    <div className="history-sidebar__item-path">
                      {revision.path}
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
        </ListViewList>
      )}
    </ListView>
  )
}
