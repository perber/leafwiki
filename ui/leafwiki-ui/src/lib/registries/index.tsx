// register sidebar panel items
import { AddPageDialog } from '@/features/page/AddPageDialog'
import { CopyPageDialog } from '@/features/page/CopyPageDialog'
import { CreatePageByPathDialog } from '@/features/page/CreatePageByPathDialog'
import { EditPageMetadataDialog } from '@/features/page/EditPageMetadataDialog'
import { MovePageDialog } from '@/features/page/MovePageDialog'
import { SortPagesDialog } from '@/features/page/SortPagesDialog'
import Search from '@/features/search/Search'
import TreeView from '@/features/tree/TreeView'
import { DialogRegistry } from '@/lib/registries/dialogRegistry'
import { PanelItemRegistry } from '@/lib/registries/panelItemRegistry'
import { FolderTree, Search as SearchIcon } from 'lucide-react'

export const panelItemRegistry = new PanelItemRegistry()
export const dialogRegistry = new DialogRegistry()

// Register sidebar panel items here

panelItemRegistry.register({
  id: 'tree',
  label: 'Tree',
  icon: () => <FolderTree size={16} />,
  render: () => {
    return <TreeView />
  },
})

panelItemRegistry.register({
  id: 'search',
  label: 'Search',
  icon: () => <SearchIcon size={16} />,
  render: () => <Search />,
})

// Register application wide dialogs here using dialogRegistry.register(...)

dialogRegistry.register({
  type: 'add',
  render: (props) => {
    return <AddPageDialog {...(props as React.ComponentProps<typeof AddPageDialog>)} />
  },
})

dialogRegistry.register({
  type: 'sort',
  render: (props) => {
    return <SortPagesDialog {...(props as React.ComponentProps<typeof SortPagesDialog>)} />
  },
})

dialogRegistry.register({
  type: 'move',
  render: (props) => {
    return <MovePageDialog {...(props as React.ComponentProps<typeof MovePageDialog>)} />
  },
})

dialogRegistry.register({
  type: 'create-by-path',
  render: (props) => {
    return <CreatePageByPathDialog {...(props as React.ComponentProps<typeof CreatePageByPathDialog>)} />
  },
})

dialogRegistry.register({
  type: 'copy-page',
  render: (props) => {
    return <CopyPageDialog {...(props as React.ComponentProps<typeof CopyPageDialog>)} />
  },
})

dialogRegistry.register({
  type: 'edit-page-metadata',
  render: (props) => {
    return <EditPageMetadataDialog {...(props as React.ComponentProps<typeof EditPageMetadataDialog>)} />
  },
})