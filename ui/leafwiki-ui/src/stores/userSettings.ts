// stores/userSettings.ts
// A logged-in user's private preferences (autoSave, language). Per-user
// server truth fetched once via GET /api/user-settings and never persisted
// to localStorage — mirrors stores/favorites.ts. App.tsx owns calling
// loadUserSettings()/clearUserSettings() as the session's logged-in state
// changes.
//
// The user's saved language is applied on top of the site-wide
// defaultLanguage (stores/config.ts) once user settings load, so it wins
// over the instance default for that user's session.

import { mapApiError } from '@/lib/api/errors'
import { getUserSettings, updateUserSettings } from '@/lib/api/userSettings'
import i18next, { getAvailableLanguages } from '@/lib/i18n'
import { create } from 'zustand'
import { toast } from 'sonner'

const t = (key: string) => i18next.t(key, { ns: 'settings' })

const DEFAULT_LANGUAGE = 'en'

function applyLanguageIfShipped(lang: string): void {
  if (getAvailableLanguages().some((language) => language.code === lang)) {
    void i18next.changeLanguage(lang)
  }
}

type UserSettingsStore = {
  autoSave: boolean
  language: string
  loaded: boolean
  loadUserSettings: () => Promise<void>
  toggleAutoSave: () => Promise<void>
  setLanguage: (language: string) => Promise<void>
  clearUserSettings: () => void
}

export const useUserSettingsStore = create<UserSettingsStore>()((set, get) => ({
  autoSave: true,
  language: DEFAULT_LANGUAGE,
  loaded: false,
  loadUserSettings: async () => {
    try {
      const settings = await getUserSettings()
      set({
        autoSave: settings.autoSave,
        language: settings.language,
        loaded: true,
      })
      applyLanguageIfShipped(settings.language)
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
  setLanguage: async (language) => {
    const previous = get().language
    if (language === previous) return
    set({ language })
    applyLanguageIfShipped(language)
    try {
      await updateUserSettings({ language })
    } catch (err) {
      set({ language: previous })
      applyLanguageIfShipped(previous)
      toast.error(
        mapApiError(err, t('account.preferences.languageChangeError')).message,
      )
    }
  },
  clearUserSettings: () =>
    set({ autoSave: true, language: DEFAULT_LANGUAGE, loaded: false }),
}))
