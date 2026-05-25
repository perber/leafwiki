import * as authAPI from '@/lib/api/auth'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type UserInfo = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

// Unix timestamp in seconds, matching the backend's use of time.Now().Unix().
type UnixTimestampSeconds = number

type SessionState = {
  isRefreshing: boolean
  accessTokenExpiresAt: UnixTimestampSeconds | null
  user: UserInfo | null
  setAccessTokenExpiresAt: (value: UnixTimestampSeconds | null) => void
  setUser: (user: UserInfo | null) => void
  setRefreshing: (value: boolean) => void
  logout: () => Promise<void>
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      user: null,
      isRefreshing: false,
      accessTokenExpiresAt: null,
      setAccessTokenExpiresAt: (value) => set({ accessTokenExpiresAt: value }),
      setRefreshing: (value) => set({ isRefreshing: value }),
      setUser: (user) => set({ user }),
      logout: async () => {
        try {
          await authAPI.logout()
        } catch (err) {
          console.warn('Logout failed:', err)
        } finally {
          set({ user: null, accessTokenExpiresAt: null })
        }
      },
    }),
    {
      name: 'session-storage',
      partialize: (state) => ({
        accessTokenExpiresAt: state.accessTokenExpiresAt,
        user: state.user,
      }),
    },
  ),
)
