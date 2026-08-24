// stores/userSettings.ts
// A logged-in user's private preferences (currently just autoSave; language
// is stored by the backend but not yet surfaced in the frontend). Per-user
// server truth fetched once via GET /api/user-settings and never persisted
// to localStorage — mirrors stores/favorites.ts. App.tsx owns calling
// loadUserSettings()/clearUserSettings() as the session's logged-in state
// changes.

import { mapApiError } from '@/lib/api/errors'
import { getUserSettings, updateUserSettings } from '@/lib/api/userSettings'
import i18next from '@/lib/i18n'
import { create } from 'zustand'
import { toast } from 'sonner'

const t = (key: string) => i18next.t(key, { ns: 'settings' })

type UserSettingsStore = {
  autoSave: boolean
  loaded: boolean
  loadUserSettings: () => Promise<void>
  toggleAutoSave: () => Promise<void>
  clearUserSettings: () => void
}

export const useUserSettingsStore = create<UserSettingsStore>()((set, get) => ({
  autoSave: true,
  loaded: false,
  loadUserSettings: async () => {
    try {
      const settings = await getUserSettings()
      set({ autoSave: settings.autoSave, loaded: true })
    } catch (err) {
      console.warn('Failed to load user settings:', err)
    }
  },
  toggleAutoSave: async () => {
    const previous = get().autoSave
    const next = !previous
    set({ autoSave: next })
    try {
      await updateUserSettings({ autoSave: next })
    } catch (err) {
      set({ autoSave: previous })
      toast.error(
        mapApiError(err, t('account.preferences.autoSaveToggleError')).message,
      )
    }
  },
  clearUserSettings: () => set({ autoSave: true, loaded: false }),
}))
