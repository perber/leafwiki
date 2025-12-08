import ScrollableContainer from '@/components/ScrollableContainer'
import { TooltipWrapper } from '@/components/TooltipWrapper'
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
          return {
            keyCombo: hotkey,
            enabled: true,
            action: actions.get(item.id)!,
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
      className="sidebar"
    >
      {/*
        The actual width is controlled by the parent container (AppLayout)
        so this element just stretches to full width.
      */}
      <div className="sidebar__inner">
        {' '}
        {/* Tab navigation */}
        <div className="sidebar__tabs">
          {/* Padding around the tabs */}
          <div className="sidebar__tabs-list">
            {tabs.map((tab) => (
              <TooltipWrapper label={tab.label} key={tab.id}>
                <button
                  key={tab.id}
                  data-testid={`sidebar-${tab.id}-tab-button`}
                  onClick={() => setSidebarMode(tab.id)}
                  className={`sidebar__tab-button ${
                    sidebarMode === tab.id
                      ? 'sidebar__tab-button--active'
                      : 'sidebar__tab-button--inactive'
                  }`}
                >
                  {tab.icon()} {tab.label}
                </button>
              </TooltipWrapper>
            ))}
          </div>
        </div>
        {/* Height 48px is the height of the tab navigation 
            so the content area takes the rest of the height
            I can't use a variable here because TailwindCSS doesn't support that
        */}
        <div className={`sidebar__content`}>
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
