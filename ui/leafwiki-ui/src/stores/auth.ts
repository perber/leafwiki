// stores/authStore.ts
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type UserInfo = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor'
}

type AuthState = {
  token: string | null
  refreshToken: string | null
  isRefreshing: boolean
  user: UserInfo | null
  setAuth: (token: string, refreshToken: string, user: UserInfo) => void
  setRefreshing: (value: boolean) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      refreshToken: null,
      user: null,
      isRefreshing: false,
      setRefreshing: (value) => set({ isRefreshing: value }),
      setAuth: (token, refreshToken, user) =>
        set({ token, refreshToken, user }),
      logout: () => set({ token: null, refreshToken: null, user: null }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        token: state.token,
        refreshToken: state.refreshToken,
        user: state.user,
      }),
    },
  ),
)
