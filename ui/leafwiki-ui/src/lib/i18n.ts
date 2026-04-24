import i18next from 'i18next'
import enErrors from '../locales/en/errors.json'

i18next.init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      errors: enErrors,
    },
  },
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
