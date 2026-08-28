// stores/userSettings.ts
// A logged-in user's private preferences (autoSave, language, date/time
// format). Per-user server truth fetched once via GET /api/user-settings and
// never persisted to localStorage — mirrors stores/favorites.ts. App.tsx owns
// calling loadUserSettings()/clearUserSettings() as the session's logged-in
// state changes.
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
const DEFAULT_DATE_FORMAT = 'locale'
const DEFAULT_TIME_FORMAT = 'locale'

function applyLanguageIfShipped(lang: string): void {
  if (getAvailableLanguages().some((language) => language.code === lang)) {
    void i18next.changeLanguage(lang)
  }
}

// Serialises the optimistic writes for one preference field. Rapid picker
// changes fire overlapping PUTs whose responses can arrive out of order:
// without this a slow earlier request could persist the older value on the
// server, and a stale failure could roll the UI back over the value the user
// actually chose (plus show a misleading error toast). Writes run in
// submission order and only the most recent one may roll back on failure.
type PrefWriter = { seq: number; chain: Promise<unknown> }

function makePrefWriter(): PrefWriter {
  return { seq: 0, chain: Promise.resolve() }
}

// Resolves `true` when this write is the latest one and its PUT succeeded
// (the caller then confirms the save), `false` when a newer write superseded
// it (the newer write owns the user feedback), and rejects when the latest
// write fails (the caller shows the error and the optimistic value is rolled
// back).
function writePref(
  writer: PrefWriter,
  apply: () => void,
  patch: () => Promise<unknown>,
  rollback: () => void,
): Promise<boolean> {
  const seq = ++writer.seq
  apply()
  writer.chain = writer.chain
    .catch(() => {})
    .then(() => patch())
    .then(() => seq === writer.seq)
    .catch((err) => {
      if (seq !== writer.seq) return false // superseded by a newer write
      rollback()
      throw err
    })
  return writer.chain as Promise<boolean>
}

const dateFormatWriter = makePrefWriter()
const timeFormatWriter = makePrefWriter()

type UserSettingsStore = {
  autoSave: boolean
  language: string
  dateFormat: string
  timeFormat: string
  loaded: boolean
  loadUserSettings: () => Promise<void>
  toggleAutoSave: () => Promise<void>
  setLanguage: (language: string) => Promise<void>
  setDateFormat: (dateFormat: string) => Promise<void>
  setTimeFormat: (timeFormat: string) => Promise<void>
  clearUserSettings: () => void
}

export const useUserSettingsStore = create<UserSettingsStore>()((set, get) => ({
  autoSave: true,
  language: DEFAULT_LANGUAGE,
  dateFormat: DEFAULT_DATE_FORMAT,
  timeFormat: DEFAULT_TIME_FORMAT,
  loaded: false,
  loadUserSettings: async () => {
    try {
      const settings = await getUserSettings()
      set({
        autoSave: settings.autoSave,
        language: settings.language,
        dateFormat: settings.dateFormat ?? DEFAULT_DATE_FORMAT,
        timeFormat: settings.timeFormat ?? DEFAULT_TIME_FORMAT,
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
      toast.success(t('account.preferences.savedToast'))
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
      toast.success(t('account.preferences.savedToast'))
    } catch (err) {
      set({ language: previous })
      applyLanguageIfShipped(previous)
      toast.error(
        mapApiError(err, t('account.preferences.languageChangeError')).message,
      )
    }
  },
  setDateFormat: async (dateFormat) => {
    const previous = get().dateFormat
    if (dateFormat === previous) return
    try {
      const applied = await writePref(
        dateFormatWriter,
        () => set({ dateFormat }),
        () => updateUserSettings({ dateFormat }),
        () => set({ dateFormat: previous }),
      )
      if (applied) toast.success(t('account.preferences.savedToast'))
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.dateFormatChangeError'))
          .message,
      )
    }
  },
  setTimeFormat: async (timeFormat) => {
    const previous = get().timeFormat
    if (timeFormat === previous) return
    try {
      const applied = await writePref(
        timeFormatWriter,
        () => set({ timeFormat }),
        () => updateUserSettings({ timeFormat }),
        () => set({ timeFormat: previous }),
      )
      if (applied) toast.success(t('account.preferences.savedToast'))
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.timeFormatChangeError'))
          .message,
      )
    }
  },
  clearUserSettings: () =>
    set({
      autoSave: true,
      language: DEFAULT_LANGUAGE,
      dateFormat: DEFAULT_DATE_FORMAT,
      timeFormat: DEFAULT_TIME_FORMAT,
      loaded: false,
    }),
}))
