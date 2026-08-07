import { Button, Checkbox, Form, Input, Space, Typography, Message } from '@arco-design/web-react'
import { BackupServerIllustration, IconCloud, IconLock, IconSafe, IconUser } from '../../components/icons'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import { LanguageSwitcher } from '../../components/common/LanguageSwitcher'
import { beginWebAuthnLogin, fetchSetupStatus, sendLoginOtp } from '../../services/auth'
import { useAuthStore } from '../../stores/auth'
import { getWebAuthnAssertion } from '../../utils/webauthn'

interface SetupFormValues {
  username: string
  password: string
  displayName: string
}

interface LoginFormValues {
  username: string
  password: string
  twoFactorCode?: string
  rememberDevice?: boolean
}

export function LoginPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const authStatus = useAuthStore((state) => state.status)
  const doLogin = useAuthStore((state) => state.login)
  const doSetup = useAuthStore((state) => state.setup)
  const [loginForm] = Form.useForm<LoginFormValues>()
  const [initialized, setInitialized] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(false)
  const [mfaActionLoading, setMfaActionLoading] = useState('')
  const [twoFactorRequired, setTwoFactorRequired] = useState(false)
  const [setupStatusFailed, setSetupStatusFailed] = useState(false)
  const setupStatusRequest = useRef(0)

  function resolveErrorMessage(error: unknown) {
    if (axios.isAxiosError(error)) {
      const code = error.response?.data?.code
      const translationKey = code ? `auth.errors.${code}` : ''
      if (translationKey && i18n.exists(translationKey)) {
        return t(translationKey)
      }
      if (i18n.resolvedLanguage === 'zh-CN' && error.response?.data?.message) {
        return error.response.data.message
      }
    }
    if (error instanceof Error && i18n.resolvedLanguage === 'zh-CN') {
      return error.message
    }
    return t('auth.requestFailed')
  }

  function resetTwoFactorPrompt() {
    if (!twoFactorRequired) {
      return
    }
    setTwoFactorRequired(false)
    loginForm.setFieldValue('twoFactorCode', undefined)
    loginForm.setFieldValue('rememberDevice', false)
  }

  useEffect(() => {
    if (authStatus === 'authenticated') {
      navigate('/dashboard', { replace: true })
    }
  }, [authStatus, navigate])

  const loadSetupStatus = useCallback(async () => {
    const requestID = ++setupStatusRequest.current
    setInitialized(null)
    setSetupStatusFailed(false)
    try {
      const result = await fetchSetupStatus()
      if (requestID === setupStatusRequest.current) {
        setInitialized(result.initialized)
      }
    } catch {
      // Do not guess that an unreachable fresh install is initialized. That
      // would hide the first-administrator form behind an impossible login.
      if (requestID === setupStatusRequest.current) {
        setSetupStatusFailed(true)
      }
    }
  }, [])

  useEffect(() => {
    void loadSetupStatus()
    return () => {
      setupStatusRequest.current++
    }
  }, [loadSetupStatus])

  const handleSetup = async (values: SetupFormValues) => {
    setLoading(true)
    try {
      await doSetup(values)
      Message.success(t('auth.setupSuccess'))
      navigate('/dashboard', { replace: true })
    } catch (error) {
      Message.error(resolveErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  const handleLogin = async (values: LoginFormValues) => {
    setLoading(true)
    try {
      await doLogin({
        ...values,
        trustedDeviceName: values.rememberDevice ? navigator.userAgent.slice(0, 120) : undefined,
      })
      setTwoFactorRequired(false)
      Message.success(t('auth.loginSuccess'))
      navigate('/dashboard', { replace: true })
    } catch (error) {
      if (axios.isAxiosError(error)) {
        const code = error.response?.data?.code
        if (code === 'AUTH_2FA_REQUIRED' || code === 'AUTH_2FA_INVALID') {
          setTwoFactorRequired(true)
          Message.error(resolveErrorMessage(error))
          return
        }
      }
      Message.error(resolveErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  function readLoginCredentials(): (LoginFormValues & { username: string; password: string }) | null {
    const values = loginForm.getFieldsValue()
    if (!values.username?.trim() || !values.password?.trim()) {
      Message.error(t('auth.credentialsRequired'))
      return null
    }
    return {
      ...values,
      username: values.username,
      password: values.password,
    }
  }

  async function handleSendOTP(channel: 'email' | 'sms') {
    const values = readLoginCredentials()
    if (!values) return
    setMfaActionLoading(channel)
    try {
      await sendLoginOtp({ username: values.username, password: values.password, channel })
      Message.success(channel === 'email' ? t('auth.emailCodeSent') : t('auth.smsCodeSent'))
    } catch (error) {
      Message.error(resolveErrorMessage(error))
    } finally {
      setMfaActionLoading('')
    }
  }

  async function handleWebAuthnLogin() {
    const values = readLoginCredentials()
    if (!values) return
    setMfaActionLoading('webauthn')
    try {
      const options = await beginWebAuthnLogin({ username: values.username, password: values.password })
      const assertion = await getWebAuthnAssertion(options)
      await doLogin({
        username: values.username,
        password: values.password,
        webAuthnAssertion: assertion,
        trustedDeviceToken: '',
        rememberDevice: values.rememberDevice,
        trustedDeviceName: navigator.userAgent.slice(0, 120),
      })
      setTwoFactorRequired(false)
      Message.success(t('auth.loginSuccess'))
      navigate('/dashboard', { replace: true })
    } catch (error) {
      Message.error(resolveErrorMessage(error))
    } finally {
      setMfaActionLoading('')
    }
  }

  const pageTitle = initialized === null
    ? t('auth.setupStatusTitle')
    : initialized
      ? t('auth.welcomeTitle')
      : t('auth.setupTitle')
  const pageSubtitle = initialized === null
    ? setupStatusFailed ? t('auth.statusErrorDescription') : t('auth.checkingStatus')
    : initialized
      ? t('auth.welcomeSubtitle')
      : t('auth.setupSubtitle')

  return (
    <div className="login-shell">
      <div className="login-bg" />
      <div className="login-container">
        <div className="login-banner">
          <div className="login-banner-inner">
            <BackupServerIllustration style={{ marginBottom: 16 }} />

            <Typography.Title heading={2} style={{ color: 'white', marginTop: 0, marginBottom: 12 }}>
              {t('auth.bannerTitle')}
            </Typography.Title>
            <Typography.Text style={{ color: 'rgba(255,255,255,0.75)', fontSize: 16 }}>
              {t('auth.bannerSubtitle')}
            </Typography.Text>
          </div>
        </div>
        
        <div className="login-form-wrapper">
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <LanguageSwitcher />
            </div>
            <div style={{ paddingBottom: 8 }}>
              <div style={{ display: 'inline-flex', alignItems: 'center', marginBottom: 16 }}>
                <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: 36, height: 36, borderRadius: 4, background: 'var(--color-primary-6)', marginRight: 12 }}>
                  <IconCloud style={{ fontSize: 20, color: 'white' }} />
                </div>
                <Typography.Title heading={4} style={{ margin: 0 }}>
                  BackupX
                </Typography.Title>
              </div>
              <Typography.Title heading={3} style={{ marginTop: 0, marginBottom: 8 }}>
                {pageTitle}
              </Typography.Title>
              <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 14 }}>
                {pageSubtitle}
              </Typography.Paragraph>
            </div>

            {initialized === null ? (
              setupStatusFailed ? (
                <div>
                  <Typography.Text>{t('auth.statusErrorTitle')}</Typography.Text>
                  <div style={{ marginTop: 12 }}>
                    <Button type="primary" loading={loading} onClick={() => void loadSetupStatus()}>
                      {t('auth.retry')}
                    </Button>
                  </div>
                </div>
              ) : (
                <Typography.Text type="secondary">{t('auth.checkingStatus')}</Typography.Text>
              )
            ) : initialized === false ? (
              <Form<SetupFormValues> layout="vertical" onSubmit={handleSetup}>
                <Form.Item field="displayName" label={t('auth.displayName')} rules={[{ required: true, minLength: 1, message: t('auth.validation.displayNameRequired') }]}>
                  <Input autoComplete="name" placeholder={t('auth.displayNamePlaceholder')} prefix={<IconUser />} size="large" />
                </Form.Item>
                <Form.Item field="username" label={t('auth.username')} rules={[
                  { required: true, message: t('auth.validation.usernameRequired') },
                  { minLength: 3, message: t('auth.validation.usernameLength') },
                ]}>
                  <Input autoComplete="username" placeholder={t('auth.usernamePlaceholder')} prefix={<IconUser />} size="large" />
                </Form.Item>
                <Form.Item field="password" label={t('auth.password')} rules={[
                  { required: true, message: t('auth.validation.passwordRequired') },
                  { minLength: 8, message: t('auth.validation.passwordLength') },
                ]}>
                  <Input.Password autoComplete="new-password" placeholder={t('auth.setupPasswordPlaceholder')} prefix={<IconLock />} size="large" />
                </Form.Item>
                <Button long type="primary" htmlType="submit" loading={loading} size="large" style={{ borderRadius: 4, height: 44, marginTop: 8 }}>
                  {t('auth.setupSubmit')}
                </Button>
              </Form>
            ) : (
              <Form<LoginFormValues> form={loginForm} layout="vertical" onSubmit={handleLogin}>
                <Form.Item field="username" label={t('auth.username')} rules={[
                  { required: true, message: t('auth.validation.usernameRequired') },
                  { minLength: 3, message: t('auth.validation.usernameLength') },
                ]}>
                  <Input autoComplete="username" placeholder={t('auth.usernamePlaceholder')} prefix={<IconUser />} size="large" onChange={resetTwoFactorPrompt} />
                </Form.Item>
                <Form.Item field="password" label={t('auth.password')} rules={[
                  { required: true, message: t('auth.validation.passwordRequired') },
                  { minLength: 8, message: t('auth.validation.passwordLength') },
                ]}>
                  <Input.Password autoComplete="current-password" placeholder={t('auth.passwordPlaceholder')} prefix={<IconLock />} size="large" onChange={resetTwoFactorPrompt} />
                </Form.Item>
                {twoFactorRequired && (
                  <>
                    <Form.Item field="twoFactorCode" label={t('auth.mfaCode')} rules={[
                      { required: true, message: t('auth.validation.mfaRequired') },
                      { minLength: 6, maxLength: 32, message: t('auth.validation.mfaLength') },
                    ]}>
                      <Input autoComplete="one-time-code" placeholder={t('auth.mfaCodePlaceholder')} prefix={<IconSafe />} size="large" maxLength={32} />
                    </Form.Item>
                    <Space wrap style={{ marginTop: -8, marginBottom: 8 }}>
                      <Button loading={mfaActionLoading === 'email'} onClick={() => void handleSendOTP('email')}>{t('auth.sendEmailCode')}</Button>
                      <Button loading={mfaActionLoading === 'sms'} onClick={() => void handleSendOTP('sms')}>{t('auth.sendSmsCode')}</Button>
                      <Button loading={mfaActionLoading === 'webauthn'} onClick={() => void handleWebAuthnLogin()}>{t('auth.usePasskey')}</Button>
                    </Space>
                    <Form.Item field="rememberDevice" triggerPropName="checked">
                      <Checkbox>{t('auth.trustDevice')}</Checkbox>
                    </Form.Item>
                  </>
                )}
                <Button long type="primary" htmlType="submit" loading={loading} size="large" style={{ borderRadius: 4, height: 44, marginTop: 16 }}>
                  {twoFactorRequired ? t('auth.verifyAndLogin') : t('auth.login')}
                </Button>
              </Form>
            )}
          </Space>
        </div>
      </div>
    </div>
  )
}
