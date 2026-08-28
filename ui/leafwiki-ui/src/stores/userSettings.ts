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
type PrefWriter = {
  // Identifies the most recent write; only it is allowed to roll back.
  seq: number
  // Bumped on login/logout: a write still queued behind a slow PUT checks
  // this and aborts rather than writing into a different user's session.
  epoch: number
  // The last value the server has actually confirmed — the rollback target.
  persisted: string
  chain: Promise<unknown>
}

function makePrefWriter(persisted: string): PrefWriter {
  return { seq: 0, epoch: 0, persisted, chain: Promise.resolve() }
}

// Points every writer at the value the server just returned and starts a
// fresh epoch, so any patch still queued from a previous session is skipped
// rather than written into the current one.
function resetPrefWriter(writer: PrefWriter, persisted: string): void {
  writer.seq = 0
  writer.epoch += 1
  writer.persisted = persisted
}

// Resolves 'saved' when this is the latest write and its PUT succeeded (the
// caller then confirms the save), 'skipped' when a newer write superseded it
// or the session changed before its turn, and rejects when the latest write
// fails — after rolling the field back to the last server-confirmed value
// (never to an earlier, still-unpersisted optimistic pick).
function writePref(
  writer: PrefWriter,
  value: string,
  apply: (v: string) => void,
  patch: (v: string) => Promise<unknown>,
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

const dateFormatWriter = makePrefWriter(DEFAULT_DATE_FORMAT)
const timeFormatWriter = makePrefWriter(DEFAULT_TIME_FORMAT)

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
      resetPrefWriter(dateFormatWriter, dateFormat)
      resetPrefWriter(timeFormatWriter, timeFormat)
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
    if (dateFormat === get().dateFormat) return
    try {
      const result = await writePref(
        dateFormatWriter,
        dateFormat,
        (v) => set({ dateFormat: v }),
        (v) => updateUserSettings({ dateFormat: v }),
      )
      if (result === 'saved') toast.success(t('account.preferences.savedToast'))
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
      if (result === 'saved') toast.success(t('account.preferences.savedToast'))
    } catch (err) {
      toast.error(
        mapApiError(err, t('account.preferences.timeFormatChangeError'))
          .message,
      )
    }
  },
  clearUserSettings: () => {
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
