import { beforeEach, describe, expect, it } from 'vitest'
import { useSidebarPanelsStore } from './sidebarPanels'

describe('useSidebarPanelsStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useSidebarPanelsStore.setState(useSidebarPanelsStore.getInitialState())
  })

  it('starts with pinned, favorites, and pages expanded by default', () => {
    expect(useSidebarPanelsStore.getState().openSections).toEqual([
      'pinned',
      'favorites',
      'pages',
    ])
  })

  it('setOpenSections replaces the open section ids', () => {
    useSidebarPanelsStore.getState().setOpenSections(['pages'])
    expect(useSidebarPanelsStore.getState().openSections).toEqual(['pages'])
  })

  it('setOpenSections can close all sections', () => {
    useSidebarPanelsStore.getState().setOpenSections([])
    expect(useSidebarPanelsStore.getState().openSections).toEqual([])
  })

  it('setOpenSections can add a new section id (e.g. a future "favorites" section)', () => {
    useSidebarPanelsStore
      .getState()
      .setOpenSections(['pinned', 'pages', 'favorites'])
    expect(useSidebarPanelsStore.getState().openSections).toEqual([
      'pinned',
      'pages',
      'favorites',
    ])
  })

  it.each([
    null,
    {},
    { openSections: null },
    { openSections: {} },
    { openSections: ['pages', 1] },
  ])('restores defaults for invalid persisted state %#', async (state) => {
    localStorage.setItem(
      'leafwiki-sidebar-panels',
      JSON.stringify({ state, version: 0 }),
    )

    await useSidebarPanelsStore.persist.rehydrate()

    expect(useSidebarPanelsStore.getState().openSections).toEqual([
      'pinned',
      'favorites',
      'pages',
    ])
    expect(useSidebarPanelsStore.getState().setOpenSections).toBeTypeOf(
      'function',
    )
  })

  it('preserves an empty persisted section list', async () => {
    localStorage.setItem(
      'leafwiki-sidebar-panels',
      JSON.stringify({ state: { openSections: [] }, version: 0 }),
    )

    await useSidebarPanelsStore.persist.rehydrate()

    expect(useSidebarPanelsStore.getState().openSections).toEqual([])
  })
})
