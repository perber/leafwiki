import i18next from 'i18next'
import HttpBackend from 'i18next-http-backend'
import { initReactI18next } from 'react-i18next'

import { API_BASE_URL } from './config'

import enAssets from '../locales/en/assets.json'
import enAuth from '../locales/en/auth.json'
import enBackup from '../locales/en/backup.json'
import enBranding from '../locales/en/branding.json'
import enCommon from '../locales/en/common.json'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'
import enHistory from '../locales/en/history.json'
import enImporter from '../locales/en/importer.json'
import enMaintenance from '../locales/en/maintenance.json'
import enPage from '../locales/en/page.json'
import enSearch from '../locales/en/search.json'
import enSidebar from '../locales/en/sidebar.json'
import enTree from '../locales/en/tree.json'
import enUsers from '../locales/en/users.json'
import enViewer from '../locales/en/viewer.json'

export const LANG_STORAGE_KEY = 'leafwiki-lang'

export const NAMESPACES = [
  'assets',
  'auth',
  'backup',
  'branding',
  'common',
  'editor',
  'errors',
  'history',
  'importer',
  'maintenance',
  'page',
  'search',
  'sidebar',
  'tree',
  'users',
  'viewer',
] as const

const bundledEnglish = {
  assets: enAssets,
  auth: enAuth,
  backup: enBackup,
  branding: enBranding,
  common: enCommon,
  editor: enEditor,
  errors: enErrors,
  history: enHistory,
  importer: enImporter,
  maintenance: enMaintenance,
  page: enPage,
  search: enSearch,
  sidebar: enSidebar,
  tree: enTree,
  users: enUsers,
  viewer: enViewer,
}

function detectLanguage(): string {
  if (typeof window === 'undefined') {
    return 'en'
  }

  const savedLanguage = window.localStorage.getItem(LANG_STORAGE_KEY)
  if (savedLanguage) {
    return savedLanguage
  }

  if (navigator.language.toLowerCase().startsWith('ru')) {
    return 'ru'
  }

  return 'en'
}

let initPromise: Promise<typeof i18next> | null = null

export function initI18n(): Promise<typeof i18next> {
  if (!initPromise) {
    initPromise = i18next
      .use(HttpBackend)
      .use(initReactI18next)
      .init({
        lng: detectLanguage(),
        fallbackLng: 'en',
        partialBundledLanguages: true,
        ns: [...NAMESPACES],
        defaultNS: 'common',
        resources: {
          en: bundledEnglish,
        },
        backend: {
          loadPath: `${API_BASE_URL}/locales/{{lng}}/{{ns}}.json`,
        },
        interpolation: {
          escapeValue: false,
        },
        react: {
          useSuspense: false,
          bindI18n: 'languageChanged loaded',
          bindI18nStore: 'added removed',
        },
      })
      .then(() => i18next)
  }

  return initPromise
}

export async function setLanguage(lang: string): Promise<void> {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(LANG_STORAGE_KEY, lang)
  }

  const currentLanguage = i18next.resolvedLanguage ?? i18next.language
  if (currentLanguage?.split('-')[0] === lang.split('-')[0]) {
    return
  }

  await i18next.changeLanguage(lang)
}

export default i18next
