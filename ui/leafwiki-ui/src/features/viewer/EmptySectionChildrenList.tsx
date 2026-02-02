import { Button } from '@/components/ui/button'
import { NODE_KIND_PAGE, NODE_KIND_SECTION, Page } from '@/lib/api/pages'
import { formatRelativeTime } from '@/lib/formatDate'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { FilePlus, FolderPlus } from 'lucide-react'
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
          <h2 className="child-list__section-title mb-1">
            Overview of the section '{page.title}'
          </h2>
          <p className="text-muted mb-4 text-sm">
            This page is the default overview of the section and lists its pages
            and sections.
            <br />
            When editing this page, you can define a custom start page for the
            section.
          </p>
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
            <div className="mb-2 flex w-full justify-end">
              <Button
                onClick={() =>
                  openDialog(DIALOG_ADD_PAGE, {
                    parentId: page.id,
                    nodeKind: NODE_KIND_PAGE,
                  })
                }
                variant="default"
                size="sm"
              >
                <FilePlus size={16} />
                Add Page
              </Button>
            </div>
          )}
        </nav>
      )}
      {/* No children - Add Button and allow users to create a new page */}
      {!hasChildren && (
        <nav
          aria-label={`Subpages of ${page.title}`}
          className="child-list__section"
        >
          <div>
            <h2 className="child-list__section-title mb-1 grow">
              This section is empty.
            </h2>
            <p className="text-muted text-sm">
              The section <b>{page.title}</b> does not contain any pages or
              sections yet. Start by adding a new page or create a new section.
            </p>
          </div>
          {!isReadOnly && (
            <div className="mb-2 flex w-full justify-end">
              <Button
                onClick={() =>
                  openDialog(DIALOG_ADD_PAGE, {
                    parentId: page.id,
                    nodeKind: NODE_KIND_PAGE,
                  })
                }
                variant="default"
                size="sm"
              >
                <FilePlus size={16} />
                Add Page
              </Button>
              <Button
                onClick={() =>
                  openDialog(DIALOG_ADD_PAGE, {
                    parentId: page.id,
                    nodeKind: NODE_KIND_SECTION,
                  })
                }
                variant="default"
                size="sm"
                className="ml-2"
              >
                <FolderPlus size={16} />
                Add Section
              </Button>
            </div>
          )}
        </nav>
      )}
    </>
  )
}
