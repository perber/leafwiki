import * as authAPI from '@/lib/api/auth'
import * as userAPI from '@/lib/api/users'
import { create } from 'zustand'
import { useAuthStore } from './auth'

type UserStore = {
  users: userAPI.User[]
  reset: () => void
  loadUsers: () => Promise<void>
  createUser: (data: Parameters<typeof userAPI.createUser>[0]) => Promise<void>
  updateUser: (data: Parameters<typeof userAPI.updateUser>[0]) => Promise<void>
  deleteUser: (id: string) => Promise<void>
  changeOwnPassword: (oldPassword: string, newPassword: string) => Promise<void>
}

export const useUserStore = create<UserStore>((set, get) => ({
  users: [],

  reset: () => set({ users: [] }),

  loadUsers: async () => {
    const users = await userAPI.getUsers()
    set({ users })
  },

  createUser: async (data) => {
    await userAPI.createUser(data)
    await get().loadUsers()
  },

  updateUser: async (data) => {
    await userAPI.updateUser(data)
    await get().loadUsers()
  },

  deleteUser: async (id) => {
    await userAPI.deleteUser(id)
    await get().loadUsers()
  },

  changeOwnPassword: async (oldPassword, newPassword) => {
    await userAPI.changeOwnPassword(oldPassword, newPassword)
    // relogin user
    const { user, logout, setAuth } = useAuthStore.getState()
    if (user && user.username) {
      try {
        const auth = await authAPI.login(user.username, newPassword)
        setAuth(auth.token, auth.refresh_token, auth.user)
      } catch (err) {
        console.warn(err)
        logout()
      }
    } else {
      logout()
    }
  },
}))
