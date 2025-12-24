import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type UserInfo = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor'
}

type SessionState = {
  isRefreshing: boolean
  user: UserInfo | null
  setUser: (user: UserInfo) => void
  setRefreshing: (value: boolean) => void
  logout: () => void
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      user: null,
      isRefreshing: false,
      setRefreshing: (value) => set({ isRefreshing: value }),
      setUser: (user) => set({ user }),
      logout: () => set({ user: null }),
    }),
    {
      name: 'session-storage',
      partialize: (state) => ({
        user: state.user,
      }),
    },
  ),
)
