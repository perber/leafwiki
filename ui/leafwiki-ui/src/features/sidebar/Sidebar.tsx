import { useSidebarStore } from '@/stores/sidebar'
import { FolderTree, Search as SearchIcon } from 'lucide-react'
import { JSX } from 'react'
import Search from '../search/Search'
import TreeView from '../tree/TreeView'

export default function Sidebar() {
  const sidebarMode = useSidebarStore((state) => state.sidebarMode)
  const setSidebarMode = useSidebarStore((state) => state.setSidebarMode)

  const tabs: { id: 'tree' | 'search'; label: string; icon: JSX.Element }[] = [
    { id: 'tree', label: 'Tree', icon: <FolderTree size={16} /> },
    { id: 'search', label: 'Search', icon: <SearchIcon size={16} /> },
  ]

  return (
    <aside key={'sidebar'} className="flex flex-1 flex-col pb-2">
      <div className="flex border-b pb-2 pl-4 pt-1 text-sm">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setSidebarMode(tab.id)}
            className={`-mb-px flex items-center gap-1 border-b-2 px-3 py-1.5 ${
              sidebarMode === tab.id
                ? 'border-green-600 font-semibold text-green-600'
                : 'border-transparent text-gray-500 hover:text-black'
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div
        className={sidebarMode === 'tree' ? 'flex flex-1 flex-col' : 'hidden'}
      >
        <TreeView />
      </div>
      <div
        className={sidebarMode === 'search' ? 'flex flex-1 flex-col' : 'hidden'}
      >
        <Search />
      </div>
    </aside>
  )
}
