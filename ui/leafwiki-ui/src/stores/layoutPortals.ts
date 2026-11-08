// stores/layoutPortals.ts
// The elements AppLayout renders for pages to portal into (subheader, TOC
// pane). Pages take them from here instead of document.getElementById:
// rendered page content may carry the same ids, and the TOC pane sits after
// the article in the DOM, so an id lookup could return an element of the page.

import { create } from 'zustand'

type LayoutPortalsStore = {
  subheaderRoot: HTMLElement | null
  tocPaneRoot: HTMLElement | null
  setSubheaderRoot: (element: HTMLElement | null) => void
  setTocPaneRoot: (element: HTMLElement | null) => void
}

export const useLayoutPortalsStore = create<LayoutPortalsStore>()((set) => ({
  subheaderRoot: null,
  tocPaneRoot: null,
  setSubheaderRoot: (subheaderRoot) => set({ subheaderRoot }),
  setTocPaneRoot: (tocPaneRoot) => set({ tocPaneRoot }),
}))
