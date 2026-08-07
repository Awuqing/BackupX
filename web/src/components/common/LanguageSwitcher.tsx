import { Select } from '@arco-design/web-react'
import { useTranslation } from 'react-i18next'
import { languageOptions, normalizeLanguage, setApplicationLanguage, type SupportedLanguage } from '../../i18n'

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation()
  const currentLanguage = normalizeLanguage(i18n.resolvedLanguage)

  return (
    <Select
      aria-label={t('auth.language')}
      value={currentLanguage}
      options={languageOptions}
      style={{ width: 120 }}
      onChange={(value) => void setApplicationLanguage(value as SupportedLanguage)}
    />
  )
}
