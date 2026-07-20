import { Accordion } from '@/components/ui/accordion'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { FavoritesSection } from '@/features/favorites/FavoritesSection'
import { SidebarAccordionSection } from '@/features/sidebar/SidebarAccordionSection'
import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { NODE_KIND_PAGE, NODE_KIND_SECTION } from '@/lib/api/pages'
import { DIALOG_ADD_PAGE, DIALOG_SORT_PAGES } from '@/lib/registries'
import { buildViewUrl } from '@/lib/routePath'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { toWikiLookupPath } from '@/lib/wikiPath'
import { useDialogsStore } from '@/stores/dialogs'
import { useFavoritesStore } from '@/stores/favorites'
import { useSessionStore } from '@/stores/session'
import { useSidebarPanelsStore } from '@/stores/sidebarPanels'
import { useTreeStore } from '@/stores/tree'
import {
  ChevronsDown,
  ChevronsUp,
  FilePlus,
  FolderPlus,
  List,
  MoreHorizontal,
  Pin,
  Star,
} from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'
import { usePageEditorStore } from '../editor/pageEditorStore'
import { PinnedSection } from './PinnedSection'
import { TreeDndProvider } from './TreeDnd'
import { TreeNode } from './TreeNode'

export default function TreeView() {
  const { t } = useTranslation('viewer')
  const tree = useTreeStore((s) => s.tree)
  const loading = useTreeStore((s) => s.loading)
  const error = useTreeStore((s) => s.error)
  const { pathname } = useLocation()
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const openAncestorsForPath = useTreeStore((s) => s.openAncestorsForPath)
  const setActiveNodeId = useTreeStore((s) => s.setActiveNodeId)
  const openNode = useTreeStore((s) => s.openNode)
  const expandAll = useTreeStore((s) => s.expandAll)
  const collapseAll = useTreeStore((s) => s.collapseAll)
  const appMode = useAppMode()
  const currentEditorPageId = usePageEditorStore(
    (state) => state.page?.id ?? state.initialPage?.id,
  )

  const currentPath = toWikiLookupPath(buildViewUrl(pathname))

  const pinnedPages = useTreeStore((s) => s.pinnedPages)
  const hasPinned = pinnedPages.length > 0
  const favoritePageIds = useFavoritesStore((s) => s.favoritePageIds)
  const isLoggedIn = useSessionStore((s) => s.user !== null)
  const hasFavorites = isLoggedIn && favoritePageIds.size > 0
  const openDialog = useDialogsStore((state) => state.openDialog)
  const readOnlyMode = useIsReadOnly()
  const openSections = useSidebarPanelsStore((s) => s.openSections)
  const setOpenSections = useSidebarPanelsStore((s) => s.setOpenSections)

  useEffect(() => {
    if (!tree || !currentPath) return
    openAncestorsForPath(currentPath)
  }, [tree, currentPath, openAncestorsForPath])

  useEffect(() => {
    if (!tree) return
    if (appMode === 'edit' && currentEditorPageId) {
      openNode(currentEditorPageId)
      setActiveNodeId(currentEditorPageId)
      return
    }

    if (!currentPath) {
      setActiveNodeId(null)
      return
    }

    const node = useTreeStore.getState().getPageByPath(currentPath)
    setActiveNodeId(node?.id ?? null)
  }, [
    tree,
    appMode,
    currentEditorPageId,
    currentPath,
    openNode,
    setActiveNodeId,
  ])

  useEffect(() => {
    if (tree === null) {
      reloadTree()
    }
  }, [tree, reloadTree])

  if (loading)
    return (
      <p className="tree-view__status tree-view__status--loading">Loading...</p>
    )

  if (error || !tree)
    return (
      <p className="tree-view__status tree-view__status--error">
        Error: {error}
      </p>
    )

  const pagesToolbar = (
    <>
      {!readOnlyMode && (
        <>
          <TreeViewActionButton
            actionName="add"
            icon={
              <FilePlus
                className="tree-view__action-icon text-brand/70!"
                size={18}
              />
            }
            tooltip="Create new page"
            onClick={() =>
              openDialog(DIALOG_ADD_PAGE, {
                parentId: '',
                nodeKind: NODE_KIND_PAGE,
              })
            }
          />
          <TreeViewActionButton
            actionName="add-section"
            icon={
              <FolderPlus
                className="tree-view__action-icon text-brand/70!"
                size={18}
              />
            }
            tooltip="Create new section"
            onClick={() =>
              openDialog(DIALOG_ADD_PAGE, {
                parentId: '',
                nodeKind: NODE_KIND_SECTION,
              })
            }
          />
        </>
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <TreeViewActionButton
            actionName="tree-overflow"
            icon={
              <MoreHorizontal className="tree-view__action-icon" size={18} />
            }
            tooltip="More actions"
          />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuItem
            className="cursor-pointer gap-2"
            onClick={expandAll}
            data-testid="tree-view-action-button-expand-all"
          >
            <ChevronsDown size={15} />
            Expand all
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer gap-2"
            onClick={collapseAll}
            data-testid="tree-view-action-button-collapse-all"
          >
            <ChevronsUp size={15} />
            Collapse all
          </DropdownMenuItem>
          {!readOnlyMode && tree && (
            <DropdownMenuItem
              className="cursor-pointer gap-2"
              onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: tree })}
              data-testid="tree-view-action-button-sort"
            >
              <List size={15} />
              Sort pages
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )

  return (
    <Accordion
      type="multiple"
      value={openSections}
      onValueChange={setOpenSections}
      className="tree-view"
    >
      {hasPinned && (
        <SidebarAccordionSection
          value="pinned"
          title={t('pinned.sectionTitle')}
          icon={<Pin size={11} />}
          collapseToggleLabel={t('pinned.togglePinnedSection')}
        >
          <PinnedSection />
        </SidebarAccordionSection>
      )}
      {hasFavorites && (
        <SidebarAccordionSection
          value="favorites"
          title={t('favorites.sectionTitle')}
          icon={<Star size={11} />}
          collapseToggleLabel={t('favorites.toggleFavoritesSection')}
        >
          <FavoritesSection />
        </SidebarAccordionSection>
      )}
      <SidebarAccordionSection
        value="pages"
        title={t('pinned.pagesSectionTitle')}
        collapseToggleLabel={t('pinned.togglePagesSection')}
        actions={pagesToolbar}
      >
        <TreeDndProvider enabled={!readOnlyMode}>
          <div className="tree-view__nodes">
            {tree?.children?.map((node) => (
              <TreeNode key={node.id} node={node} />
            ))}
          </div>
        </TreeDndProvider>
      </SidebarAccordionSection>
    </Accordion>
  )
}
