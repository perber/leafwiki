import i18next from 'i18next'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'
import enViewer from '../locales/en/viewer.json'

i18next.init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      errors: enErrors,
      editor: enEditor,
      viewer: enViewer,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
