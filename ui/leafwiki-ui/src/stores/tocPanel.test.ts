import { beforeEach, describe, expect, it } from 'vitest'
import { useTocPanelStore } from './tocPanel'

describe('useTocPanelStore', () => {
  beforeEach(() => {
    useTocPanelStore.setState({ collapsed: false })
  })

  it('starts expanded by default', () => {
    expect(useTocPanelStore.getState().collapsed).toBe(false)
  })

  it('setCollapsed sets the collapsed flag explicitly', () => {
    useTocPanelStore.getState().setCollapsed(true)
    expect(useTocPanelStore.getState().collapsed).toBe(true)

    useTocPanelStore.getState().setCollapsed(false)
    expect(useTocPanelStore.getState().collapsed).toBe(false)
  })

  it('toggleCollapsed flips the current state', () => {
    useTocPanelStore.getState().toggleCollapsed()
    expect(useTocPanelStore.getState().collapsed).toBe(true)

    useTocPanelStore.getState().toggleCollapsed()
    expect(useTocPanelStore.getState().collapsed).toBe(false)
  })
})
