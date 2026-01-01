import * as authAPI from '@/lib/api/auth'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type UserInfo = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

type SessionState = {
  isRefreshing: boolean
  user: UserInfo | null
  setUser: (user: UserInfo | null) => void
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
      logout: async () => {
        try {
          await authAPI.logout()
        } catch (err) {
          console.warn('Logout failed:', err)
        } finally {
          set({ user: null })
        }
      },
    }),
    {
      name: 'session-storage',
      partialize: (state) => ({
        user: state.user,
      }),
    },
  ),
)
