import i18n, { normalizeLanguage, setApplicationLanguage } from './i18n'

describe('application language', () => {
  afterEach(async () => {
    await setApplicationLanguage('zh-CN')
  })

  it('normalizes supported English variants', () => {
    expect(normalizeLanguage('en')).toBe('en-US')
    expect(normalizeLanguage('en-GB')).toBe('en-US')
    expect(normalizeLanguage('zh-CN')).toBe('zh-CN')
  })

  it('persists the language selected before login', async () => {
    await setApplicationLanguage('en-US')

    expect(localStorage.getItem('backupx-language')).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(i18n.t('auth.setupTitle')).toBe('System setup')
  })
})
