// progressbar state
// contains the progress value and visibility state

import { create } from 'zustand'

export interface ProgressbarState {
  loading: boolean
  setLoading: (loading: boolean) => void

}

export const useProgressbarStore = create<ProgressbarState>((set) => ({
  loading: false,
  displayProgressbar: false,
  setLoading: (loading: boolean) => {
    set({ loading })
  },
}))