import { Button } from '@/components/ui/button'
import { NODE_KIND_SECTION, Page } from '@/lib/api/pages'
import { formatRelativeTime } from '@/lib/formatDate'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { FilePlus } from 'lucide-react'
import { Link } from 'react-router-dom'

type EmptySectionChildrenListProps = {
  page: Page
}

function displayUser(label?: { username: string }) {
  return label?.username || null
}

export default function EmptySectionChildrenList({
  page,
}: EmptySectionChildrenListProps) {
  const getPageById = useTreeStore((s) => s.getPageById)
  const node = getPageById(page.id)
  const openDialog = useDialogsStore((s) => s.openDialog)
  const tree = useTreeStore((s) => s.tree)
  const isReadOnly = useIsReadOnly()

  if (!tree) {
    return null
  }

  if (page.kind !== NODE_KIND_SECTION) {
    return null
  }

  // If the page has content, do not show the child list
  if (page.content && page.content.trim().length > 0) {
    return null
  }

  if (!node) {
    return null
  }

  const hasChildren = node.children && node.children.length > 0

  return (
    <>
      {hasChildren && (
        <nav
          aria-label={`Subpages of ${page.title}`}
          className="child-list__section"
        >
          <h2 className="child-list__section-title">
            Pages and Sections in '{page.title}'
          </h2>
          <ul>
            {node.children?.map((n) => {
              if (!n) return null

              const editorName = displayUser(n?.metadata?.lastAuthor)
              const updatedRelative = formatRelativeTime(n?.metadata?.updatedAt)

              return (
                <li key={n.id}>
                  <Link to={`/${n.path}`}>{n.title}</Link>{' '}
                  {n.kind === NODE_KIND_SECTION && ' (Section)'}
                  <br />
                  {/* Last edited info */}
                  <span className="text-muted text-sm">
                    {' '}
                    Updated{' '}
                    {editorName
                      ? `by ${editorName} · ${updatedRelative}`
                      : updatedRelative}
                  </span>
                </li>
              )
            })}
          </ul>
          {!isReadOnly && (
            <Button
              onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: page.id })}
              variant="default"
              size="sm"
            >
              <FilePlus size={16} />
              Add Page
            </Button>
          )}
        </nav>
      )}
      {/* No children - Add Button and allow users to create a new page */}
      {!hasChildren && (
        <nav
          aria-label={`Subpages of ${page.title}`}
          className="child-list__section"
        >
          <div className="mb-2 flex items-center justify-between">
            <h2 className="child-list__section-title grow">
              No Pages and Sections in '{page.title}'
            </h2>
          </div>
          {!isReadOnly && (
            <Button
              onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: page.id })}
              variant="default"
              size="sm"
            >
              <FilePlus size={16} />
              Add Page
            </Button>
          )}
        </nav>
      )}
    </>
  )
}
