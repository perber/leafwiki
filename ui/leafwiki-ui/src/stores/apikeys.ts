import * as apiKeyAPI from '@/lib/api/apikeys'
import { create } from 'zustand'

type ApiKeyStore = {
  apiKeys: apiKeyAPI.ApiKey[]
  reset: () => void
  loadApiKeys: () => Promise<void>
  createApiKey: (
    data: Parameters<typeof apiKeyAPI.createApiKey>[0],
  ) => Promise<apiKeyAPI.CreateApiKeyResult>
  deleteApiKey: (id: string) => Promise<void>
}

export const useApiKeyStore = create<ApiKeyStore>((set, get) => ({
  apiKeys: [],

  reset: () => set({ apiKeys: [] }),

  loadApiKeys: async () => {
    const apiKeys = await apiKeyAPI.getApiKeys()
    set({ apiKeys })
  },

  createApiKey: async (data) => {
    const result = await apiKeyAPI.createApiKey(data)
    await get().loadApiKeys()
    return result
  },

  deleteApiKey: async (id) => {
    await apiKeyAPI.deleteApiKey(id)
    await get().loadApiKeys()
  },
}))
