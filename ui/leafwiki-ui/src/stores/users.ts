import * as api from "@/lib/api"
import { create } from "zustand"

type UserStore = {
  users: api.User[]
  loadUsers: () => Promise<void>
  createUser: (data: Parameters<typeof api.createUser>[0]) => Promise<void>
  updateUser: (data: Parameters<typeof api.updateUser>[0]) => Promise<void>
  deleteUser: (id: string) => Promise<void>
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
}))
