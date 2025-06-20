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
    <aside key={'sidebar'}>
      {/*
        Our sidebar has always the same width, so we can use a fixed width.
        This will help us to avoid layout shifts when the sidebar is toggled.
        I can't use w-96 because it would add a scrollbar, because the container above is adding a border-right.
      */}
      <div className="w-385px">
        <div className="pt-2 p-4 pb-2"> {/* Padding Container */}
          <div className="flex border-b text-sm">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setSidebarMode(tab.id)}
                className={`-mb-px flex items-center gap-1 border-b-2 px-3 py-1.5 ${sidebarMode === tab.id
                  ? 'border-green-600 font-semibold text-green-600'
                  : 'border-transparent text-gray-500 hover:text-black'
                  }`}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </div>
        </div>
        <div className="pl-4 pr-4">
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
        </div>
      </div>
    </aside>
  )
}
