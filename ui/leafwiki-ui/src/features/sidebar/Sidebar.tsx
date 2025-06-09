import { useSidebarStore } from '@/stores/sidebar'
import { FolderTree, Search as SearchIcon } from 'lucide-react'
import Search from '../search/Search'
import TreeView from '../tree/TreeView'

export default function Sidebar() {
  const sidebarMode = useSidebarStore((state) => state.sidebarMode)
  const setSidebarMode = useSidebarStore((state) => state.setSidebarMode)

  const tabs = [
    { id: 'tree', label: 'Tree', icon: <FolderTree size={16} /> },
    { id: 'search', label: 'Search', icon: <SearchIcon size={16} /> },
  ]

  return (
    <aside key={'sidebar'}>
      <h2 className="mb-4 text-xl font-bold">🌿 LeafWiki</h2>
      <div className="mb-4 flex border-b text-sm">
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
      <div className={sidebarMode === 'tree' ? 'block' : 'hidden'}>
        <TreeView />
      </div>
      <div className={sidebarMode === 'search' ? 'block' : 'hidden'}>
        <Search />
      </div>
    </aside>
  )
}
