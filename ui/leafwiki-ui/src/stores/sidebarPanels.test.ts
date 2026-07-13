import { beforeEach, describe, expect, it } from 'vitest'
import { useSidebarPanelsStore } from './sidebarPanels'

describe('useSidebarPanelsStore', () => {
  beforeEach(() => {
    useSidebarPanelsStore.setState({ openSections: ['pinned', 'pages'] })
  })

  it('starts with pinned and pages expanded by default', () => {
    expect(useSidebarPanelsStore.getState().openSections).toEqual([
      'pinned',
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
})
