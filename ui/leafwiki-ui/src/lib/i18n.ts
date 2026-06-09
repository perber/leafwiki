import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'
import enSearch from '../locales/en/search.json'
import enViewer from '../locales/en/viewer.json'

i18next.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      errors: enErrors,
      editor: enEditor,
      search: enSearch,
      viewer: enViewer,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
