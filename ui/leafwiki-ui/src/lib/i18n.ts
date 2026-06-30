import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import enAuth from '../locales/en/auth.json'
import enBackup from '../locales/en/backup.json'
import enBranding from '../locales/en/branding.json'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'
import enMaintenance from '../locales/en/maintenance.json'
import enSearch from '../locales/en/search.json'
import enViewer from '../locales/en/viewer.json'

i18next.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      auth: enAuth,
      backup: enBackup,
      branding: enBranding,
      errors: enErrors,
      editor: enEditor,
      maintenance: enMaintenance,
      search: enSearch,
      viewer: enViewer,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
