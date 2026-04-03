import { createI18n } from 'vue-i18n'
import en from './en.json'
import ru from './ru.json'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en, ru },
})

export default i18n

export function setLocale(locale: string): void {
  const supported = ['en', 'ru']
  const resolved = supported.includes(locale) ? locale : 'en'
  i18n.global.locale.value = resolved as 'en' | 'ru'
  localStorage.setItem('locale', resolved)
}

export function getSavedLocale(): string | null {
  return localStorage.getItem('locale')
}
