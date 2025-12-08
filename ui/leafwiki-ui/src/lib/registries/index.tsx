// register sidebar panel items
import { UnsavedChangesDialog } from '@/components/UnsavedChangesDialog'
import { AssetManagerDialog } from '@/features/assets/AssetManagerDialog'
import { BacklinkPane } from '@/features/backlinks/BacklinkPane'
import { OutlinePane } from '@/features/outline/OutlinePane'
import { AddPageDialog } from '@/features/page/AddPageDialog'
import { CopyPageDialog } from '@/features/page/CopyPageDialog'
import { CreatePageByPathDialog } from '@/features/page/CreatePageByPathDialog'
import { DeletePageDialog } from '@/features/page/DeletePageDialog'
import { EditPageMetadataDialog } from '@/features/page/EditPageMetadataDialog'
import { MovePageDialog } from '@/features/page/MovePageDialog'
import { SortPagesDialog } from '@/features/page/SortPagesDialog'
import Search from '@/features/search/Search'
import TreeView from '@/features/tree/TreeView'
import { ChangeOwnPasswordDialog } from '@/features/users/ChangeOwnPasswordDialog'
import { ChangePasswordDialog } from '@/features/users/ChangePasswordDialog'
import { DeleteUserDialog } from '@/features/users/DeleteUserDialog'
import { UserFormDialog } from '@/features/users/UserFormDialog'
import { DialogRegistry } from '@/lib/registries/dialogRegistry'
import { PanelItemRegistry } from '@/lib/registries/panelItemRegistry'
import { FolderTree, ListTree, Search as SearchIcon, Undo2 } from 'lucide-react'

export const panelItemRegistry = new PanelItemRegistry()
export const dialogRegistry = new DialogRegistry()

// Register sidebar panel items here

export const SIDEBAR_TREE_PANEL_ID = 'tree'
export const SIDEBAR_SEARCH_PANEL_ID = 'search'
export const SIDEBAR_BACKLINKS_PANEL_ID = 'backlinks'
export const SIDEBAR_OUTLINE_PANEL_ID = 'outline'

panelItemRegistry.register({
  id: SIDEBAR_TREE_PANEL_ID,
  label: 'Explorer',
  hotkey: 'Mod+Shift+E',
  icon: () => <FolderTree size={16} />,
  render: () => {
    return <TreeView />
  },
})

panelItemRegistry.register({
  id: SIDEBAR_SEARCH_PANEL_ID,
  label: 'Search',
  hotkey: 'Mod+Shift+F',
  icon: () => <SearchIcon size={16} />,
  render: (props: unknown) => {
    const SearchProps = props as React.ComponentProps<typeof Search>
    return <Search {...SearchProps} />
  },
})

panelItemRegistry.register({
  id: SIDEBAR_BACKLINKS_PANEL_ID,
  label: 'Backlinks',
  hotkey: 'Mod+Shift+B',
  icon: () => <Undo2 size={16} />,
  render: () => {
    return <BacklinkPane />
  },
})

panelItemRegistry.register({
  id: SIDEBAR_OUTLINE_PANEL_ID,
  label: 'Outline',
  hotkey: 'Mod+Shift+O',
  icon: () => <ListTree size={16} />, // irgendein Icon
  render: () => <OutlinePane />,
})

// Register application wide dialogs here using dialogRegistry.register(...)

export const DIALOG_ADD_PAGE = 'add-page'
export const DIALOG_SORT_PAGES = 'sort-pages'
export const DIALOG_MOVE_PAGE = 'move-page'
export const DIALOG_CREATE_PAGE_BY_PATH = 'create-page-by-path'
export const DIALOG_COPY_PAGE = 'copy-page'
export const DIALOG_EDIT_PAGE_METADATA = 'edit-page-metadata'
export const DIALOG_ASSET_MANAGER = 'asset-manager'
export const DIALOG_DELETE_PAGE_CONFIRMATION = 'delete-page-confirmation'
export const DIALOG_CHANGE_OWN_PASSWORD = 'change-own-password'
export const DIALOG_USER_FORM = 'user-form'
export const DIALOG_CHANGE_USER_PASSWORD = 'change-user-password'
export const DIALOG_DELETE_USER_CONFIRMATION = 'delete-user-confirmation'
export const DIALOG_UNSAVED_CHANGES = 'unsaved-changes'

dialogRegistry.register({
  type: DIALOG_ADD_PAGE,
  render: (props) => {
    return (
      <AddPageDialog
        key={DIALOG_ADD_PAGE}
        {...(props as React.ComponentProps<typeof AddPageDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_SORT_PAGES,
  render: (props) => {
    return (
      <SortPagesDialog
        key={DIALOG_SORT_PAGES}
        {...(props as React.ComponentProps<typeof SortPagesDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_MOVE_PAGE,
  render: (props) => {
    return (
      <MovePageDialog
        key={DIALOG_MOVE_PAGE}
        {...(props as React.ComponentProps<typeof MovePageDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_CREATE_PAGE_BY_PATH,
  render: (props) => {
    return (
      <CreatePageByPathDialog
        key={DIALOG_CREATE_PAGE_BY_PATH}
        {...(props as React.ComponentProps<typeof CreatePageByPathDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_COPY_PAGE,
  render: (props) => {
    return (
      <CopyPageDialog
        key={DIALOG_COPY_PAGE}
        {...(props as React.ComponentProps<typeof CopyPageDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_EDIT_PAGE_METADATA,
  render: (props) => {
    return (
      <EditPageMetadataDialog
        key={DIALOG_EDIT_PAGE_METADATA}
        {...(props as React.ComponentProps<typeof EditPageMetadataDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_ASSET_MANAGER,
  render: (props) => {
    return (
      <AssetManagerDialog
        key={DIALOG_ASSET_MANAGER}
        {...(props as React.ComponentProps<typeof AssetManagerDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_DELETE_PAGE_CONFIRMATION,
  render: (props) => {
    return (
      <DeletePageDialog
        key={DIALOG_DELETE_PAGE_CONFIRMATION}
        {...(props as React.ComponentProps<typeof DeletePageDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_CHANGE_OWN_PASSWORD,
  render: () => {
    return <ChangeOwnPasswordDialog key={DIALOG_CHANGE_OWN_PASSWORD} />
  },
})

dialogRegistry.register({
  type: DIALOG_USER_FORM,
  render: (props) => {
    return (
      <UserFormDialog
        key={DIALOG_USER_FORM}
        {...(props as React.ComponentProps<typeof UserFormDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_CHANGE_USER_PASSWORD,
  render: (props) => {
    return (
      <ChangePasswordDialog
        key={DIALOG_CHANGE_USER_PASSWORD}
        {...(props as React.ComponentProps<typeof ChangePasswordDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_DELETE_USER_CONFIRMATION,
  render: (props) => {
    return (
      <DeleteUserDialog
        key={DIALOG_DELETE_USER_CONFIRMATION}
        {...(props as React.ComponentProps<typeof DeleteUserDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_UNSAVED_CHANGES,
  render: (props) => {
    return (
      <UnsavedChangesDialog
        key={DIALOG_UNSAVED_CHANGES}
        {...(props as React.ComponentProps<typeof UnsavedChangesDialog>)}
      />
    )
  },
})
