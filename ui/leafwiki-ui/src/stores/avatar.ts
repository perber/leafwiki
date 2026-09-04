// stores/avatar.ts
// Self-service profile-picture state: just a cache-busting version counter
// plus the upload/delete actions. Deliberately separate from
// stores/userSettings.ts (language/autosave preferences) — an avatar is
// profile identity, not an editing preference, and there's no prior client
// state to optimistically roll back on failure (unlike userSettings' toggle),
// so this mirrors stores/branding.ts's try/catch/rethrow shape instead.

import * as avatarAPI from '@/lib/api/avatar'
import i18next from '@/lib/i18n'
import { create } from 'zustand'

const t = (key: string) => i18next.t(key, { ns: 'settings' })

type AvatarStore = {
  avatarVersion: number
  isLoading: boolean
  error: string | null

  uploadAvatar: (file: File) => Promise<void>
  deleteAvatar: () => Promise<void>
}

export const useAvatarStore = create<AvatarStore>((set) => ({
  avatarVersion: 0,
  isLoading: false,
  error: null,

  uploadAvatar: async (file) => {
    set({ isLoading: true, error: null })
    try {
      await avatarAPI.uploadAvatar(file)
      set({ avatarVersion: Date.now() })
    } catch (err) {
      set({
        error:
          err instanceof Error ? err.message : t('account.avatar.uploadFailed'),
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },

  deleteAvatar: async () => {
    set({ isLoading: true, error: null })
    try {
      await avatarAPI.deleteAvatar()
      set({ avatarVersion: Date.now() })
    } catch (err) {
      set({
        error:
          err instanceof Error ? err.message : t('account.avatar.deleteFailed'),
      })
      throw err
    } finally {
      set({ isLoading: false })
    }
  },
}))
