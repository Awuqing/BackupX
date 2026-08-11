import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import zhCN from './locales/zh-CN.json'
import enUS from './locales/en-US.json'

export type SupportedLanguage = 'zh-CN' | 'en-US'

export const languageOptions: Array<{ label: string; value: SupportedLanguage }> = [
  { label: '中文', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
]

export function normalizeLanguage(value?: string | null): SupportedLanguage {
  return value?.toLowerCase().startsWith('en') ? 'en-US' : 'zh-CN'
}

const savedLanguage = normalizeLanguage(typeof window === 'undefined' ? null : window.localStorage.getItem('backupx-language'))

if (typeof document !== 'undefined') {
  document.documentElement.lang = savedLanguage
}

i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
    'en-US': { translation: enUS },
  },
  lng: savedLanguage,
  fallbackLng: 'zh-CN',
  interpolation: {
    escapeValue: false,
  },
})

export async function setApplicationLanguage(language: SupportedLanguage) {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem('backupx-language', language)
  }
  if (typeof document !== 'undefined') {
    document.documentElement.lang = language
  }
  await i18n.changeLanguage(language)
}

export default i18n
