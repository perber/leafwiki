import ScrollableContainer from '@/components/ScrollableContainer'
import { TooltipWrapper } from '@/components/TooltipWrapper'
import { panelItemRegistry } from '@/lib/registries'
import { createHotkeyDefinition } from '@/lib/shortcuts/shortcutCatalog'
import { useAppMode } from '@/lib/useAppMode'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useSidebarStore } from '@/stores/sidebar'
import { JSX, Suspense, useEffect, useMemo } from 'react'

const registeredItems = panelItemRegistry.getAllItems()
const sidebarShortcutIds: Partial<
  Record<string, 'sidebar.explorer.open' | 'sidebar.search.open'>
> = {
  tree: 'sidebar.explorer.open',
  search: 'sidebar.search.open',
}

export default function Sidebar() {
  const appMode = useAppMode()
  const sidebarMode = useSidebarStore((state) => state.sidebarMode)
  const setSidebarMode = useSidebarStore((state) => state.setSidebarMode)
  const setSidebarVisible = useSidebarStore((state) => state.setSidebarVisible)

  const items = useMemo(
    () =>
      registeredItems.filter((item) => {
        if (item.modes && !item.modes.includes(appMode)) return false
        if (item.isEnabled && !item.isEnabled()) return false
        return true
      }),
    [appMode],
  )

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

  useEffect(() => {
    if (items.length === 0) return

    const hasActiveItem = items.some((item) => item.id === sidebarMode)
    if (!hasActiveItem) {
      setSidebarMode(items[0].id)
    }
  }, [items, setSidebarMode, sidebarMode])

  // add hotkeys for each tab
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  // Create stable action functions outside of the map using useMemo
  // This prevents actions from being recreated on every render
  const actions = useMemo(() => {
    const actionMap = new Map<string, () => void>()
    items.forEach((item) => {
      actionMap.set(item.id, () => {
        setSidebarVisible(true)
        setSidebarMode(item.id)
      })
    })
    return actionMap
  }, [items, setSidebarMode, setSidebarVisible])

  // Memoize hotkey definitions using the stable actions
  const hotKeyDefs = useMemo(
    () =>
      items
        .map((item) => {
          const shortcutId = sidebarShortcutIds[item.id]
          if (!shortcutId) return null

          return createHotkeyDefinition(shortcutId, actions.get(item.id)!)
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
        {tabs.length > 0 ? (
          <div className="sidebar__tabs">
            <div className="sidebar__tabs-list">
              {tabs.map((tab) => (
                <TooltipWrapper
                  label={tab.label}
                  key={tab.id}
                  parentClassName="min-w-0"
                >
                  <button
                    data-testid={`sidebar-${tab.id}-tab-button`}
                    onClick={() => setSidebarMode(tab.id)}
                    className={`sidebar__tab-button ${
                      sidebarMode === tab.id
                        ? 'sidebar__tab-button--active'
                        : 'sidebar__tab-button--inactive'
                    }`}
                  >
                    {tab.icon()} <span className="truncate">{tab.label}</span>
                  </button>
                </TooltipWrapper>
              ))}
            </div>
          </div>
        ) : null}
        <div className={`sidebar__content`}>
          {items.map((item) => (
            <ScrollableContainer key={item.id} hidden={sidebarMode !== item.id}>
              <Suspense fallback={null}>
                {item.render({ active: sidebarMode === item.id })}
              </Suspense>
            </ScrollableContainer>
          ))}
        </div>
      </div>
    </aside>
  )
}
