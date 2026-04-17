import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from '@/locales/en.json' with { type: 'json' }
import es from '@/locales/es.json' with { type: 'json' }

// Flat namespace — all keys live under the "translation" default namespace.
// Keys are grouped by section in the JSON (common.*, settings.*, lobby.*, etc.)
// and accessed as t('settings.title'), t('game.yourTurn'), etc.

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    es: { translation: es },
  },
  lng: 'en',
  fallbackLng: 'en',
  interpolation: {
    escapeValue: false, // React already escapes
  },
})

export { i18n }
