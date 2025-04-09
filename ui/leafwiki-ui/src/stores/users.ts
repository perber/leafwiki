import * as api from '@/lib/api'
import { create } from 'zustand'
import { useAuthStore } from './auth'

type UserStore = {
  users: api.User[]
  loadUsers: () => Promise<void>
  createUser: (data: Parameters<typeof api.createUser>[0]) => Promise<void>
  updateUser: (data: Parameters<typeof api.updateUser>[0]) => Promise<void>
  deleteUser: (id: string) => Promise<void>
  changeOwnPassword: (oldPassword: string, newPassword: string) => Promise<void>
}

export const useUserStore = create<UserStore>((set, get) => ({
  users: [],

  loadUsers: async () => {
    const users = await api.getUsers()
    set({ users })
  },

  createUser: async (data) => {
    await api.createUser(data)
    await get().loadUsers()
  },

  updateUser: async (data) => {
    await api.updateUser(data)
    await get().loadUsers()
  },

  deleteUser: async (id) => {
    await api.deleteUser(id)
    await get().loadUsers()
  },

  changeOwnPassword: async (oldPassword, newPassword) => {
    await api.changeOwnPassword(oldPassword, newPassword)
    // relogin user
    const { user, logout, setAuth } = useAuthStore.getState()
    if (user && user.username) {
      try {
        const auth = await api.login(user.username, newPassword)
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
