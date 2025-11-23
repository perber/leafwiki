import ScrollableContainer from '@/components/ScrollableContainer'
import { panelItemRegistry } from '@/lib/registries'
import { PanelItem } from '@/lib/registries/panelItemRegistry'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useSidebarStore } from '@/stores/sidebar'
import { JSX, useEffect, useMemo } from 'react'

const registeredItems = panelItemRegistry.getAllItems()

export default function Sidebar() {
  const sidebarMode = useSidebarStore((state) => state.sidebarMode)
  const setSidebarMode = useSidebarStore((state) => state.setSidebarMode)

  const items = registeredItems

  const tabs: { id: string; label: string; icon: () => JSX.Element }[] =
    useMemo(
      () =>
        items.map((item) => ({
          id: item.id,
          label: item.label,
          icon: item.icon,
        })),
      [items],
    )

  // add hotkeys for each tab
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  // Create stable action functions outside of the map using useMemo
  // This prevents actions from being recreated on every render
  const actions = useMemo(() => {
    const actionMap = new Map<string, () => void>()
    items.forEach((item) => {
      actionMap.set(item.id, () => setSidebarMode(item.id))
    })
    return actionMap
  }, [items, setSidebarMode])

  // Memoize hotkey definitions using the stable actions
  const hotKeyDefs = useMemo(
    () =>
      items
        .map((item) => {
          const hotkey = (item as PanelItem).hotkey as string | undefined
          if (!hotkey) return null
          const action = actions.get(item.id)
          if (!action) return null
          return {
            keyCombo: hotkey,
            enabled: true,
            action,
            mode: ['view', 'edit'],
          }
        })
        .filter(Boolean) as {
        keyCombo: string
        enabled: boolean
        action: () => void
        mode: string[]
      }[],
    [items, actions],
  )

  useEffect(() => {
    hotKeyDefs.forEach((hotKeyDef) => {
      registerHotkey(hotKeyDef)
    })
    return () => {
      hotKeyDefs.forEach((hotKeyDef) => {
        unregisterHotkey(hotKeyDef.keyCombo)
      })
    }
  }, [hotKeyDefs, registerHotkey, unregisterHotkey])

  return (
    <aside
      key={'sidebar'}
      data-testid="sidebar"
      id="sidebar"
      className="flex h-full w-full flex-col overflow-hidden bg-white"
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
                {tab.icon()}
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
          {items.map((item) => (
            <ScrollableContainer key={item.id} hidden={sidebarMode !== item.id}>
              {item.render({ active: sidebarMode === item.id })}
            </ScrollableContainer>
          ))}
        </div>
      </div>
    </aside>
  )
}
