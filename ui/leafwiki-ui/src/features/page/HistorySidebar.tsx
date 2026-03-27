import {
  ListView,
  ListViewItem,
  ListViewList,
  ListViewStatus,
} from '@/components/ListView'
import { Button } from '@/components/ui/button'
import { formatRelativeTime } from '@/lib/formatDate'
import { Loader2 } from 'lucide-react'
import { loadMorePageHistory, usePageHistoryStore } from './pageHistory'

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

function displayAuthor(username?: string, authorId?: string) {
  return username || authorId || 'Unknown'
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

  return (
    <ListView
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
          {revisions.map((revision) => {
            const selected = revision.id === selectedRevisionId
            return (
              <ListViewItem
                key={revision.id}
                active={selected}
                className="history-sidebar__item"
                onClick={() => selectRevision(revision.id)}
                testId={`history-sidebar-revision-${revision.id}`}
              >
                <div className="history-sidebar__item-row">
                  <span className="history-sidebar__item-type">
                    {revisionTypeLabel(revision.type)}
                  </span>
                  <span className="history-sidebar__item-time">
                    {formatRelativeTime(revision.createdAt) ||
                      revision.createdAt}
                  </span>
                </div>
                <div className="history-sidebar__item-author">
                  {displayAuthor(revision.author?.username, revision.authorId)}
                </div>
                {revision.summary ? (
                  <div className="history-sidebar__item-summary">
                    {revision.summary}
                  </div>
                ) : null}
              </ListViewItem>
            )
          })}

          {nextCursor ? (
            <Button
              variant="outline"
              className="w-full"
              onClick={() => void loadMorePageHistory()}
              disabled={loadingMore}
            >
              {loadingMore ? 'Loading...' : 'Load more'}
            </Button>
          ) : null}
        </ListViewList>
      )}
    </ListView>
  )
}
