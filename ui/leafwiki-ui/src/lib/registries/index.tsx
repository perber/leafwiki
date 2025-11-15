// register sidebar panel items
import Search from '@/features/search/Search'
import TreeView from '@/features/tree/TreeView'
import { PanelItemRegistry } from '@/lib/registries/panelItemRegistry'
import { FolderTree, Search as SearchIcon } from 'lucide-react'

export const panelItemRegistry = new PanelItemRegistry()

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
