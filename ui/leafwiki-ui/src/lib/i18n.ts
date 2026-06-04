import i18next from 'i18next'
import enEditor from '../locales/en/editor.json'
import enErrors from '../locales/en/errors.json'

i18next.init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      errors: enErrors,
      editor: enEditor,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
