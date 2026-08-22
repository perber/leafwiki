import '@/lib/i18n'
import { useHotKeysStore } from '@/stores/hotkeys'
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useToolbarStore } from './toolbarStore'
import { useExitModeButton } from './useExitModeButton'

beforeEach(() => {
  useToolbarStore.setState({ buttons: [] })
  useHotKeysStore.setState({ registeredHotkeys: {} })
})

describe('useExitModeButton', () => {
  it('registers a single outline back button on mount', () => {
    const onExit = vi.fn()

    renderHook(() =>
      useExitModeButton({
        id: 'exit-settings',
        labelKey: 'exit',
        ns: 'settings',
        hotkeyId: 'settings.exit',
        onExit,
      }),
    )

    const buttons = useToolbarStore.getState().buttons
    expect(buttons).toHaveLength(1)
    expect(buttons[0]).toMatchObject({
      id: 'exit-settings',
      label: 'Exit settings',
      variant: 'outline',
    })
  })

  it('fires onExit when the matching hotkey action is invoked', () => {
    const onExit = vi.fn()

    renderHook(() =>
      useExitModeButton({
        id: 'exit-settings',
        labelKey: 'exit',
        ns: 'settings',
        hotkeyId: 'settings.exit',
        onExit,
      }),
    )

    const registered = useHotKeysStore.getState().registeredHotkeys['Escape']
    expect(registered).toHaveLength(1)
    registered[0].action()

    expect(onExit).toHaveBeenCalledTimes(1)
  })

  it('cleans up the toolbar button and hotkey on unmount', () => {
    const onExit = vi.fn()

    const { unmount } = renderHook(() =>
      useExitModeButton({
        id: 'exit-settings',
        labelKey: 'exit',
        ns: 'settings',
        hotkeyId: 'settings.exit',
        onExit,
      }),
    )

    unmount()

    expect(useToolbarStore.getState().buttons).toHaveLength(0)
    expect(
      useHotKeysStore.getState().registeredHotkeys['Escape'],
    ).toBeUndefined()
  })
})
