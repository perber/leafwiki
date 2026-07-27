import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import enApiKeys from '../locales/en/apikeys.json'
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
      assets: enAssets,
      auth: enAuth,
      backup: enBackup,
      branding: enBranding,
      common: enCommon,
      errors: enErrors,
      editor: enEditor,
      history: enHistory,
      importer: enImporter,
      maintenance: enMaintenance,
      page: enPage,
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
