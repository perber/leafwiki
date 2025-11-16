// register sidebar panel items
import { AssetManagerDialog } from '@/features/assets/AssetManagerDialog'
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
import { UserFormDialog } from '@/features/users/UserFormDialog'
import { DialogRegistry } from '@/lib/registries/dialogRegistry'
import { PanelItemRegistry } from '@/lib/registries/panelItemRegistry'
import { FolderTree, Search as SearchIcon } from 'lucide-react'

export const panelItemRegistry = new PanelItemRegistry()
export const dialogRegistry = new DialogRegistry()

// Register sidebar panel items here

export const SIDEBAR_TREE_PANEL_ID = 'tree'
export const SIDEBAR_SEARCH_PANEL_ID = 'search'

panelItemRegistry.register({
  id: SIDEBAR_TREE_PANEL_ID,
  label: 'Tree',
  icon: () => <FolderTree size={16} />,
  render: () => {
    return <TreeView />
  },
})

panelItemRegistry.register({
  id: SIDEBAR_SEARCH_PANEL_ID,
  label: 'Search',
  icon: () => <SearchIcon size={16} />,
  render: () => <Search />,
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

dialogRegistry.register({
  type: DIALOG_ADD_PAGE,
  render: (props) => {
    return (
      <AddPageDialog
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
        {...(props as React.ComponentProps<typeof DeletePageDialog>)}
      />
    )
  },
})

dialogRegistry.register({
  type: DIALOG_CHANGE_OWN_PASSWORD,
  render: () => {
    return <ChangeOwnPasswordDialog />
  },
})

dialogRegistry.register({
  type: DIALOG_USER_FORM,
  render: (props) => {
    return (
      <UserFormDialog
        {...(props as React.ComponentProps<typeof UserFormDialog>)}
      />
    )
  },
})
