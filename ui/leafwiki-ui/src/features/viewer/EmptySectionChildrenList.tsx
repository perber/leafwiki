import { Button } from '@/components/ui/button'
import { NODE_KIND_PAGE, NODE_KIND_SECTION, Page } from '@/lib/api/pages'
import i18next from '@/lib/i18n'
import { formatRelativeTime } from '@/lib/formatDate'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { FilePlus, FolderPlus } from 'lucide-react'
import { Link } from 'react-router-dom'

const t = (key: string, opts?: object) =>
  i18next.t(key, { ns: 'viewer', ...opts })

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
            {t('section.overviewTitle', { title: page.title })}
          </h2>
          <p className="text-muted mb-4 text-sm">
            {isReadOnly ? (
              t('section.overviewDescriptionReadOnly')
            ) : (
              <>
                {t('section.overviewDescriptionBase')}
                <br />
                {t('section.overviewDescriptionEditorHint')}
              </>
            )}
          </p>
          <ul>
            {node.children?.map((n) => {
              if (!n) return null

              const editorName = displayUser(n?.metadata?.lastAuthor)
              const updatedRelative = formatRelativeTime(n?.metadata?.updatedAt)

              return (
                <li key={n.id}>
                  <Link to={`/${n.path}`} state={createNavigationVisitState()}>
                    {n.title}
                  </Link>{' '}
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
                {t('section.addPage')}
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
              {t('section.emptyTitle')}
            </h2>
            <p className="text-muted text-sm">
              {isReadOnly
                ? t('section.emptyDescriptionReadOnly', { title: page.title })
                : t('section.emptyDescriptionEditor', { title: page.title })}
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
                {t('section.addPage')}
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
                {t('section.addSection')}
              </Button>
            </div>
          )}
        </nav>
      )}
    </>
  )
}
