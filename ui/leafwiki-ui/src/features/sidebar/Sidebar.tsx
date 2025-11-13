import ScrollableContainer from '@/components/ScrollableContainer'
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
    <aside
      key={'sidebar'}
      data-testid="sidebar"
      className="sidebar-container flex h-full w-full flex-col overflow-hidden bg-white"
    >
      {/*
        The actual width is controlled by the parent container (AppLayout)
        so this element just stretches to full width.
      */}
      <div className="block h-full w-full">
        {' '}
        {/* Tab navigation */}
        <div className="tab-navigation border-b bg-gray-50 p-2">
          {/* Padding around the tabs */}
          <div className="flex text-sm">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                data-testid={`sidebar-${tab.id}-tab-button`}
                onClick={() => setSidebarMode(tab.id)}
                className={`-mb-px flex items-center gap-1 px-3 py-1.5 ${
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
        </div>
        {/* Height 48px is the height of the tab navigation 
            so the content area takes the rest of the height
            I can't use a variable here because TailwindCSS doesn't support that
        */}
        <div className={`sidebar-content h-[calc(100%-48px)] w-full`}>
          {/* Content */}
          <ScrollableContainer hidden={sidebarMode !== 'tree'}>
            <TreeView />
          </ScrollableContainer>
          <ScrollableContainer hidden={sidebarMode !== 'search'}>
            <Search />
          </ScrollableContainer>
        </div>
      </div>
    </aside>
  )
}
