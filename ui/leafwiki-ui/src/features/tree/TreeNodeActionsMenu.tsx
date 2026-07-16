import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  applyPageRefactor,
  convertPage,
  getPageByPath,
  NODE_KIND_PAGE,
  NODE_KIND_SECTION,
  pinPage,
  previewPageRefactor,
  updatePage,
} from '@/lib/api/pages'
import type { Page, PageNode } from '@/lib/api/pages'
import { asApiLocalizedError, mapApiError } from '@/lib/api/errors'
import {
  DIALOG_ADD_PAGE,
  DIALOG_COPY_PAGE,
  DIALOG_DELETE_PAGE_CONFIRMATION,
  DIALOG_EDIT_PAGE_METADATA,
  DIALOG_MOVE_PAGE,
  DIALOG_SORT_PAGES,
} from '@/lib/registries'
import { stripBasePath } from '@/lib/routePath'
import { getDeleteRedirectRoutePath } from '@/lib/wikiPath'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useViewerStore } from '@/features/viewer/viewer'
import { useTreeStore } from '@/stores/tree'
import {
  Copy,
  FilePlus,
  FolderPlus,
  List,
  MoreVertical,
  Move,
  Pencil,
  Pin,
  PinOff,
  Repeat2,
  Trash,
} from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useItemLabels } from '@/lib/useItemLabels'
import { usePageEditorStore } from '../editor/pageEditorStore'
import { confirmPageRefactor } from '../page/pageRefactorDialogState'
import { TreeViewActionButton } from './TreeViewActionButton'
import { useTreeNodeActionsMenusStore } from './treeNodeActionsMenus'

export type TreeNodeActionsMenuProps = {
  node: PageNode
}

export default function TreeNodeActionsMenu({
  node,
}: TreeNodeActionsMenuProps) {
  const { t } = useTranslation('tree')
  const { t: tViewer } = useTranslation('viewer')
  const { id: nodeId, kind: nodeKind, children, version: nodeVersion } = node
  const { itemCapitalized } = useItemLabels(nodeKind)
  const currentEditorPageId = usePageEditorStore((state) => state.page?.id)
  const enableLinkRefactor = useConfigStore((s) => s.enableLinkRefactor)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const reloadTree = useTreeStore((state) => state.reloadTree)
  const hasChildren = children && children.length > 0
  const navigate = useNavigate()
  const setOpenMenuNodeId = useTreeNodeActionsMenusStore(
    (s) => s.setOpenMenuNodeId,
  )
  const open = useTreeNodeActionsMenusStore((s) => s.openMenuNodeId === node.id)

  const handleConvertPage = useCallback(() => {
    convertPage(
      nodeId,
      nodeKind === NODE_KIND_PAGE ? NODE_KIND_SECTION : NODE_KIND_PAGE,
      nodeVersion,
    )
      .then(() => {
        toast.success(t('toast.converted'))
        reloadTree()
      })
      .catch((err) => {
        const localized = asApiLocalizedError(err)
        if (localized?.code === 'page_version_conflict') {
          reloadTree()
          const viewerPage = useViewerStore.getState().page
          if (viewerPage?.id === nodeId && viewerPage.path) {
            useViewerStore
              .getState()
              .loadPageData(viewerPage.path)
              .catch(console.error)
          }
          toast.error(t('toast.versionConflict'))
        } else {
          const mapped = mapApiError(err, t('toast.convertFailed'))
          toast.error(mapped.message)
        }
      })
  }, [nodeId, nodeKind, nodeVersion, reloadTree, t])

  const setPinnedLocally = useTreeStore((s) => s.setPinnedLocally)

  const getCurrentRoutePath = useCallback(() => {
    if (typeof window === 'undefined') {
      return '/'
    }
    return window.location.pathname
  }, [])

  const handleTogglePin = useCallback(() => {
    const newPinned = !node.pinned
    pinPage(nodeId, nodeVersion, newPinned)
      .then((updated) => {
        setPinnedLocally(nodeId, newPinned, updated.version)
        toast.success(
          node.pinned ? tViewer('pinned.unpinSuccess') : tViewer('pinned.pinSuccess'),
        )
      })
      .catch(() => toast.error(tViewer('pinned.pinError')))
  }, [nodeId, nodeVersion, node.pinned, setPinnedLocally, tViewer])

  const handleRenamePage = useCallback(
    async (title: string, slug: string) => {
      if (currentEditorPageId === nodeId) {
        toast.warning(
          t('toast.editingRenameBlocked', { item: itemCapitalized }),
        )
        return
      }

      try {
        const page = await getPageByPath(node.path)
        const titleChanged = page.title !== title
        const slugChanged = page.slug !== slug

        if (!titleChanged && !slugChanged) {
          return
        }

        let updatedPage: Page | null

        if (slugChanged && enableLinkRefactor) {
          const preview = await previewPageRefactor(page.id, {
            kind: 'rename',
            title,
            slug,
          })
          const rewriteLinks = await confirmPageRefactor(preview, {
            allowSkipRewrite: true,
          })

          if (rewriteLinks === null) {
            return
          }

          updatedPage = await applyPageRefactor(page.id, {
            kind: 'rename',
            version: page.version,
            title,
            slug,
            content: page.content,
            rewriteLinks,
          })
        } else {
          updatedPage = await updatePage(
            page.id,
            page.version,
            title,
            slug,
            page.content,
            page.tags ?? [],
            page.properties ?? {},
          )
        }

        await reloadTree()

        const viewerPage = useViewerStore.getState().page
        if (viewerPage?.id === nodeId && updatedPage) {
          useViewerStore.setState({
            page: {
              ...viewerPage,
              ...updatedPage,
              tags: updatedPage.tags ?? page.tags ?? [],
              properties: updatedPage.properties ?? page.properties ?? {},
            },
            notFound: false,
            error: null,
          })
        }

        const currentRoutePath = getCurrentRoutePath()
        const currentRouterPath =
          stripBasePath(currentRoutePath) ?? currentRoutePath
        if (currentRouterPath === `/${node.path}` && updatedPage?.path) {
          navigate(`/${updatedPage.path}`)
        }

        toast.success(t('toast.renamed', { item: itemCapitalized }))
      } catch (err) {
        const localized = asApiLocalizedError(err)
        if (localized?.code === 'page_version_conflict') {
          await reloadTree()
          toast.error(t('toast.versionConflict'))
          return
        }

        const mapped = mapApiError(err, t('toast.renameFailed'))
        toast.error(mapped.message)
      }
    },
    [
      currentEditorPageId,
      enableLinkRefactor,
      getCurrentRoutePath,
      navigate,
      node.path,
      nodeId,
      nodeKind,
      itemCapitalized,
      t,
    ],
  )

  return (
    <DropdownMenu
      open={open}
      onOpenChange={(nextOpen) => setOpenMenuNodeId(nextOpen ? node.id : null)}
    >
      <DropdownMenuTrigger asChild aria-label={t('moreActions')}>
        <TreeViewActionButton
          actionName="open-more-actions"
          icon={<MoreVertical size={18} className="tree-node__action-icon" />}
          tooltip={t('openMoreActions')}
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            openDialog(DIALOG_ADD_PAGE, {
              parentId: nodeId,
              nodeKind: NODE_KIND_PAGE,
            })
          }}
        >
          <FilePlus size={18} className="tree-node__action-icon" />{' '}
          {t('actions.addPage')}
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            openDialog(DIALOG_ADD_PAGE, {
              parentId: nodeId,
              nodeKind: NODE_KIND_SECTION,
            })
          }}
        >
          <FolderPlus size={18} className="tree-node__action-icon" />{' '}
          {t('actions.addSection')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            navigate(`/e/${node.path}`)
          }}
        >
          <Pencil size={18} className="tree-node__action-icon" />{' '}
          {t('actions.editItem', { item: itemCapitalized })}
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer"
          data-testid="tree-view-action-button-rename"
          onClick={() => {
            openDialog(DIALOG_EDIT_PAGE_METADATA, {
              parentId: node.parentId ?? '',
              currentId: node.id,
              itemKind: node.kind,
              title: node.title,
              slug: node.slug,
              onChange: handleRenamePage,
            })
          }}
        >
          <Pencil size={18} className="tree-node__action-icon" />{' '}
          {t('actions.renameItem', { item: itemCapitalized })}
        </DropdownMenuItem>
        {nodeKind === NODE_KIND_PAGE && (
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => {
              openDialog(DIALOG_COPY_PAGE, { sourcePage: node })
            }}
          >
            <Copy size={18} className="tree-node__action-icon" />{' '}
            {t('actions.copyPage')}
          </DropdownMenuItem>
        )}
        {hasChildren && (
          <DropdownMenuItem
            className="cursor-pointer"
            data-testid="tree-view-action-button-sort"
            onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: node })}
          >
            <List size={18} className="tree-node__action-icon" />{' '}
            {t('actions.sortChildren', { item: itemCapitalized })}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          className="cursor-pointer"
          data-testid="tree-view-action-button-move"
          onClick={() => openDialog(DIALOG_MOVE_PAGE, { pageId: node.id })}
        >
          <Move size={18} className="tree-node__action-icon" />{' '}
          {t('actions.moveItem', { item: itemCapitalized })}
        </DropdownMenuItem>
        {nodeKind === NODE_KIND_SECTION && !hasChildren && (
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={handleConvertPage}
          >
            <Repeat2 size={18} className="tree-node__action-icon" />{' '}
            {t('actions.convertToPage')}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="cursor-pointer"
          data-testid="tree-view-action-button-pin"
          onClick={handleTogglePin}
        >
          {node.pinned ? (
            <>
              <PinOff size={18} className="tree-node__action-icon" />{' '}
              {tViewer('pinned.unpinPage')}
            </>
          ) : (
            <>
              <Pin size={18} className="tree-node__action-icon" />{' '}
              {tViewer('pinned.pinPage')}
            </>
          )}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-error cursor-pointer"
          data-testid="tree-view-action-button-delete"
          onClick={() => {
            const currentRoutePath = getCurrentRoutePath()
            const currentRouterPath =
              stripBasePath(currentRoutePath) ?? currentRoutePath
            const isCurrentlyEditedNode =
              currentRouterPath.startsWith('/e/') &&
              currentEditorPageId === node.id

            if (isCurrentlyEditedNode) {
              toast.warning(
                t('toast.editingDeleteBlocked', { item: itemCapitalized }),
              )
              return
            }

            openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, {
              pageId: node?.id,
              redirectTo: getDeleteRedirectRoutePath(
                currentRouterPath,
                node.path,
              ),
            })
          }}
        >
          <Trash size={18} className="tree-node__action-icon text-error" />{' '}
          {t('actions.deleteItem', { item: itemCapitalized })}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
