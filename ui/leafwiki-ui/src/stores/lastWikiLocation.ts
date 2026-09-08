// stores/lastWikiLocation.ts
// Remembers the last non-Settings route the user was on, so leaving the
// Settings page can return there instead of redirecting to `/` (which
// RootRedirect turns into "the first wiki entry"). `state` is kept because it
// carries the `leafwikiVisitId` used by useScrollRestoration — re-supplying it
// on exit lets the previous scroll position be restored too.

import { create } from 'zustand'

export type StoredWikiLocation = {
  pathname: string
  search: string
  state: unknown
}

type LastWikiLocationStore = {
  location: StoredWikiLocation | null
  setLocation: (location: StoredWikiLocation | null) => void
}

export const useLastWikiLocationStore = create<LastWikiLocationStore>(
  (set) => ({
    location: null,
    setLocation: (location) => set({ location }),
  }),
)
