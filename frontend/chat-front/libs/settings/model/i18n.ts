import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import ru from './translations/ru'
import en from './translations/en'

i18n
    .use(initReactI18next)
    .init({
        resources: {
            ru: { translation: ru },
            en: { translation: en },
        },
        lng: localStorage.getItem('lang') ?? 'ru',
        fallbackLng: 'ru',
        interpolation: {
            escapeValue: false,
        },
    })

export default i18n