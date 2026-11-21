import { beforeEach, describe, expect, it, vi } from 'vitest'
import { HotKeyDefinition, useHotKeysStore } from './hotkeys'

function hotkey(keyCombo: string): HotKeyDefinition {
  return { keyCombo, enabled: true, mode: ['view'], action: vi.fn() }
}

function actionsFor(keyCombo: string) {
  return useHotKeysStore
    .getState()
    .registeredHotkeys[keyCombo]?.map((entry) => entry.action)
}

describe('useHotKeysStore', () => {
  beforeEach(() => {
    useHotKeysStore.setState({ registeredHotkeys: {} })
  })

  it('stacks hotkeys for a combo with the last registered on top', () => {
    const first = hotkey('Escape')
    const second = hotkey('Escape')
    const { registerHotkey } = useHotKeysStore.getState()

    registerHotkey(first)
    registerHotkey(second)

    expect(actionsFor('Escape')).toEqual([first.action, second.action])
  })

  it('removes the caller’s own hotkey and keeps a later one on top', () => {
    const below = hotkey('Escape')
    const top = hotkey('Escape')
    const { registerHotkey, unregisterHotkey } = useHotKeysStore.getState()
    registerHotkey(below)
    registerHotkey(top)

    unregisterHotkey(below)

    expect(actionsFor('Escape')).toEqual([top.action])
  })

  it('removes hotkeys out of order across combos', () => {
    const escapeA = hotkey('Escape')
    const enter = hotkey('Enter')
    const escapeB = hotkey('Escape')
    const { registerHotkey, unregisterHotkey } = useHotKeysStore.getState()
    registerHotkey(escapeA)
    registerHotkey(enter)
    registerHotkey(escapeB)

    unregisterHotkey(escapeA)
    unregisterHotkey(enter)

    expect(actionsFor('Escape')).toEqual([escapeB.action])
    expect(useHotKeysStore.getState().registeredHotkeys).not.toHaveProperty(
      'Enter',
    )
  })

  it('removes a hotkey registered under a combo that needed normalizing', () => {
    const definition = hotkey('Shift + Ctrl + KeyK')
    const { registerHotkey, unregisterHotkey } = useHotKeysStore.getState()
    registerHotkey(definition)

    unregisterHotkey(definition)

    expect(useHotKeysStore.getState().registeredHotkeys).toEqual({})
  })

  it.each([
    { name: 'never registered', registered: [] as string[] },
    { name: 'already removed', registered: ['Escape'] },
  ])('ignores unregistering a hotkey that is $name', ({ registered }) => {
    const definition = hotkey('Escape')
    const other = hotkey('Escape')
    const { registerHotkey, unregisterHotkey } = useHotKeysStore.getState()
    registerHotkey(other)
    if (registered.length > 0) {
      registerHotkey(definition)
      unregisterHotkey(definition)
    }

    unregisterHotkey(definition)

    expect(actionsFor('Escape')).toEqual([other.action])
  })
})
