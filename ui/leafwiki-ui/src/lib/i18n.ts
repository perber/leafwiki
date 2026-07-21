import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import enApiKeys from '../locales/en/apikeys.json'
import enAuth from '../locales/en/auth.json'
import enBackup from '../locales/en/backup.json'
import enBranding from '../locales/en/branding.json'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'
import enMaintenance from '../locales/en/maintenance.json'
import enRestore from '../locales/en/restore.json'
import enSearch from '../locales/en/search.json'
import enSnapshot from '../locales/en/snapshot.json'
import enUsers from '../locales/en/users.json'
import enViewer from '../locales/en/viewer.json'

i18next.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      apikeys: enApiKeys,
      auth: enAuth,
      backup: enBackup,
      branding: enBranding,
      errors: enErrors,
      editor: enEditor,
      maintenance: enMaintenance,
      restore: enRestore,
      search: enSearch,
      snapshot: enSnapshot,
      users: enUsers,
      viewer: enViewer,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
