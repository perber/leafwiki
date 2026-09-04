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

// Every preference field persists through one of these. It serialises the
// optimistic writes so overlapping, out-of-order PUTs can't leave the server
// on a stale value, and it keeps just enough state to fail safely:
type PrefWriter<T> = {
  // Identifies the most recent write; only it is allowed to roll back.
  seq: number
  // Bumped on login/logout: a write still queued behind a slow PUT checks
  // this and aborts rather than writing into a different user's session.
  epoch: number
  // The last value the server has actually confirmed — the rollback target.
  persisted: T
  chain: Promise<unknown>
}

function makePrefWriter<T>(persisted: T): PrefWriter<T> {
  return { seq: 0, epoch: 0, persisted, chain: Promise.resolve() }
}

// Points a writer at the value the server just returned and starts a fresh
// epoch, so any patch still queued from a previous session is skipped rather
// than written into the current one. Called on both login and logout.
function resetPrefWriter<T>(writer: PrefWriter<T>, persisted: T): void {
  writer.seq = 0
  writer.epoch += 1
  writer.persisted = persisted
}

// Resolves 'saved' when this is the latest write and its PUT succeeded (the
// caller then confirms the save), 'skipped' when a newer write superseded it
// or the session changed before its turn, and rejects when the latest write
// fails — after rolling the field back to the last server-confirmed value
// (never to an earlier, still-unpersisted optimistic pick). `apply` is reused
// for the rollback, so it must carry every side effect of showing the value
// (e.g. switching the i18n language), not just the store write.
function writePref<T>(
  writer: PrefWriter<T>,
  value: T,
  apply: (v: T) => void,
  patch: (v: T) => Promise<unknown>,
): Promise<'saved' | 'skipped'> {
  const seq = ++writer.seq
  const epoch = writer.epoch
  apply(value)
  writer.chain = writer.chain
    .catch(() => {})
    .then(async () => {
      if (writer.epoch !== epoch) return 'skipped' as const // session changed
      await patch(value)
      if (writer.epoch !== epoch) return 'skipped' as const // session changed mid-PUT
      writer.persisted = value
      return seq === writer.seq ? ('saved' as const) : ('skipped' as const)
    })
    .catch((err) => {
      // superseded, or the session changed: the newer write / new session
      // owns the visible state, so leave it alone.
      if (writer.epoch !== epoch || seq !== writer.seq)
        return 'skipped' as const
      apply(writer.persisted)
      throw err
    })
  return writer.chain as Promise<'saved' | 'skipped'>
}

const autoSaveWriter = makePrefWriter<boolean>(true)
const languageWriter = makePrefWriter<string>(DEFAULT_LANGUAGE)
const dateFormatWriter = makePrefWriter<string>(DEFAULT_DATE_FORMAT)
const timeFormatWriter = makePrefWriter<string>(DEFAULT_TIME_FORMAT)

const savedToast = () => toast.success(t('account.preferences.savedToast'))

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
      const dateFormat = settings.dateFormat ?? DEFAULT_DATE_FORMAT
      const timeFormat = settings.timeFormat ?? DEFAULT_TIME_FORMAT
      set({
        autoSave: settings.autoSave,
        language: settings.language,
        dateFormat,
        timeFormat,
        loaded: true,
      })
      resetPrefWriter(autoSaveWriter, settings.autoSave)
      resetPrefWriter(languageWriter, settings.language)
      resetPrefWriter(dateFormatWriter, dateFormat)
      resetPrefWriter(timeFormatWriter, timeFormat)
      applyLanguageIfShipped(settings.language)
    } catch (err) {
      console.warn('Failed to load user settings:', err)
    }
  },
  toggleAutoSave: async () => {
    try {
      const result = await writePref(
        autoSaveWriter,
        !get().autoSave,
        (v) => set({ autoSave: v }),
        (v) => updateUserSettings({ autoSave: v }),
      )
      if (result === 'saved') savedToast()
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.autoSaveToggleError')).message,
      )
    }
  },
  setLanguage: async (language) => {
    if (language === get().language) return
    try {
      const result = await writePref(
        languageWriter,
        language,
        (v) => {
          set({ language: v })
          applyLanguageIfShipped(v)
        },
        (v) => updateUserSettings({ language: v }),
      )
      if (result === 'saved') savedToast()
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.languageChangeError')).message,
      )
    }
  },
  setDateFormat: async (dateFormat) => {
    if (dateFormat === get().dateFormat) return
    try {
      const result = await writePref(
        dateFormatWriter,
        dateFormat,
        (v) => set({ dateFormat: v }),
        (v) => updateUserSettings({ dateFormat: v }),
      )
      if (result === 'saved') savedToast()
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.dateFormatChangeError'))
          .message,
      )
    }
  },
  setTimeFormat: async (timeFormat) => {
    if (timeFormat === get().timeFormat) return
    try {
      const result = await writePref(
        timeFormatWriter,
        timeFormat,
        (v) => set({ timeFormat: v }),
        (v) => updateUserSettings({ timeFormat: v }),
      )
      if (result === 'saved') savedToast()
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.timeFormatChangeError'))
          .message,
      )
    }
  },
  clearUserSettings: () => {
    resetPrefWriter(autoSaveWriter, true)
    resetPrefWriter(languageWriter, DEFAULT_LANGUAGE)
    resetPrefWriter(dateFormatWriter, DEFAULT_DATE_FORMAT)
    resetPrefWriter(timeFormatWriter, DEFAULT_TIME_FORMAT)
    set({
      autoSave: true,
      language: DEFAULT_LANGUAGE,
      dateFormat: DEFAULT_DATE_FORMAT,
      timeFormat: DEFAULT_TIME_FORMAT,
      loaded: false,
    })
  },
}))
