import { TooltipProvider } from '@/components/ui/tooltip'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useSidebarStore } from '@/stores/sidebar'
import { act, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Sidebar from './Sidebar'

vi.hoisted(() => {
  const store = new Map<string, string>()
  const localStorage = {
    get length() {
      return store.size
    },
    clear: () => {
      store.clear()
    },
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }

  Object.defineProperty(globalThis, 'localStorage', {
    value: localStorage,
    configurable: true,
  })
})

function renderSidebar() {
  render(
    <MemoryRouter initialEntries={['/docs/getting-started']}>
      <TooltipProvider>
        <Sidebar />
      </TooltipProvider>
    </MemoryRouter>,
  )
}

function invokeHotkey(combo: string) {
  const hotkeys = useHotKeysStore.getState().registeredHotkeys[combo]
  const hotkey = hotkeys[hotkeys.length - 1]

  expect(hotkey).toBeDefined()

  act(() => {
    hotkey?.action()
  })
}

describe('Sidebar', () => {
  beforeEach(() => {
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: false,
      media: '(max-width: 767px)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))

    useSidebarStore.setState({ sidebarVisible: false, sidebarMode: 'search' })
    useHotKeysStore.setState({ registeredHotkeys: {} })
  })

  it('opens the collapsed sidebar when the explorer hotkey is invoked', () => {
    renderSidebar()

    invokeHotkey('Mod+Shift+KeyE')

    expect(useSidebarStore.getState().sidebarVisible).toBe(true)
    expect(useSidebarStore.getState().sidebarMode).toBe('tree')
  })

  it('opens the collapsed sidebar when the search hotkey is invoked', () => {
    useSidebarStore.setState({ sidebarVisible: false, sidebarMode: 'tree' })

    renderSidebar()

    invokeHotkey('Mod+Shift+KeyF')

    expect(useSidebarStore.getState().sidebarVisible).toBe(true)
    expect(useSidebarStore.getState().sidebarMode).toBe('search')
  })

  it('keeps the sidebar open when switching panels by hotkey', () => {
    useSidebarStore.setState({ sidebarVisible: true, sidebarMode: 'tree' })

    renderSidebar()

    invokeHotkey('Mod+Shift+KeyF')

    expect(useSidebarStore.getState().sidebarVisible).toBe(true)
    expect(useSidebarStore.getState().sidebarMode).toBe('search')
  })
})
