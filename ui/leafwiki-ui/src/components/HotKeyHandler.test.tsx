import { TooltipProvider } from '@/components/ui/tooltip'
import Sidebar from '@/features/sidebar/Sidebar'
import { useDialogsStore } from '@/stores/dialogs'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useSidebarStore } from '@/stores/sidebar'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { HotKeyHandler } from './HotKeyHandler'

vi.mock('@/features/tree/TreeView', () => ({
  default: () => <div data-testid="tree-view-stub" />,
}))

vi.mock('@/lib/api/tags', () => ({
  fetchTags: vi.fn().mockResolvedValue([]),
}))

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

function renderApp() {
  render(
    <MemoryRouter initialEntries={['/docs/getting-started']}>
      <TooltipProvider>
        <HotKeyHandler />
        <Sidebar />
      </TooltipProvider>
    </MemoryRouter>,
  )
}

describe('HotKeyHandler', () => {
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

    useSidebarStore.setState({ sidebarVisible: true, sidebarMode: 'search' })
    useHotKeysStore.setState({ registeredHotkeys: {} })
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
  })

  it('registers the keydown listener in the capture phase', () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    render(
      <MemoryRouter initialEntries={['/docs/getting-started']}>
        <HotKeyHandler />
      </MemoryRouter>,
    )
    expect(addSpy).toHaveBeenCalledWith('keydown', expect.any(Function), true)
    addSpy.mockRestore()
  })

  it('prevents the default browser action for Ctrl+Alt+P when registered', () => {
    const action = vi.fn()
    useHotKeysStore.getState().registerHotkey({
      keyCombo: 'Mod+Alt+KeyP',
      enabled: true,
      mode: ['view'],
      action,
    })

    render(
      <MemoryRouter initialEntries={['/docs/getting-started']}>
        <HotKeyHandler />
      </MemoryRouter>,
    )

    const event = new KeyboardEvent('keydown', {
      key: 'p',
      code: 'KeyP',
      ctrlKey: true,
      altKey: true,
      bubbles: true,
      cancelable: true,
    })
    window.dispatchEvent(event)

    expect(action).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('switches to the Explorer panel on Ctrl+Shift+E while the search field is focused', async () => {
    renderApp()

    const searchInput = await screen.findByTestId('search-input')
    searchInput.focus()
    expect(searchInput).toHaveFocus()

    fireEvent.keyDown(searchInput, {
      key: 'e',
      code: 'KeyE',
      ctrlKey: true,
      shiftKey: true,
    })

    await waitFor(() => {
      expect(useSidebarStore.getState().sidebarMode).toBe('tree')
    })
  })
})
