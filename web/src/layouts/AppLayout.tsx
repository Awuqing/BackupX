import { Alert, Avatar, Button, Divider, Dropdown, Layout, Menu, Message, Modal, Form, Input, Space, Tag, Typography } from '@arco-design/web-react'
import {
  IconDashboard,
  IconStorage,
  IconFile,
  IconHistory,
  IconRefresh,
  IconSafe,
  IconCopy,
  IconBook,
  IconUser,
  IconCommand,
  IconNotification,
  IconSettings,
  IconMenuFold,
  IconMenuUnfold,
  IconLock,
  IconPoweroff,
  IconDown,
  IconCloud,
  IconDesktop,
  IconList,
} from '@arco-design/web-react/icon'
import { useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  changePassword,
  beginWebAuthnRegistration,
  clearTrustedDeviceToken,
  configureOtp,
  deleteWebAuthnCredential,
  disableTwoFactor,
  enableTwoFactor,
  finishWebAuthnRegistration,
  listTrustedDevices,
  listWebAuthnCredentials,
  prepareTwoFactor,
  regenerateRecoveryCodes,
  revokeTrustedDevice,
  type ChangePasswordPayload,
  type TrustedDevice,
  type UserInfo,
  type WebAuthnCredential,
  type TwoFactorSetupResult,
} from '../services/auth'
import { createWebAuthnCredential } from '../utils/webauthn'
import { useAuthStore } from '../stores/auth'
import { resolveErrorMessage } from '../utils/error'
import { isAdmin, roleLabel } from '../utils/permissions'
import { GlobalSearch } from '../components/common/GlobalSearch'
import { EventCenter } from '../components/common/EventCenter'

const Header = Layout.Header
const Sider = Layout.Sider
const Content = Layout.Content

function resolveSelectedKey(pathname: string) {
  if (pathname.startsWith('/backup/tasks')) {
    return '/backup/tasks'
  }
  if (pathname.startsWith('/backup/records')) {
    return '/backup/records'
  }
  if (pathname.startsWith('/restore/records')) {
    return '/restore/records'
  }
  if (pathname.startsWith('/verify/records')) {
    return '/verify/records'
  }
  if (pathname.startsWith('/replication/records')) {
    return '/replication/records'
  }
  if (pathname.startsWith('/storage-targets')) {
    return '/storage-targets'
  }
  if (pathname.startsWith('/settings/notifications')) {
    return '/settings/notifications'
  }
  if (pathname.startsWith('/audit')) {
    return '/audit'
  }
  if (pathname.startsWith('/nodes')) {
    return '/nodes'
  }
  if (pathname.startsWith('/task-templates')) {
    return '/task-templates'
  }
  if (pathname.startsWith('/admin/users')) {
    return '/admin/users'
  }
  if (pathname.startsWith('/admin/api-keys')) {
    return '/admin/api-keys'
  }
  if (pathname.startsWith('/settings') || pathname.startsWith('/system-info')) {
    return '/settings'
  }
  return pathname
}

interface MenuItemConfig {
  key: string
  label: string
  icon: React.ReactNode
  adminOnly?: boolean
}

const menuItems: MenuItemConfig[] = [
  { key: '/dashboard', label: '仪表盘', icon: <IconDashboard /> },
  { key: '/backup/tasks', label: '备份任务', icon: <IconFile /> },
  { key: '/backup/records', label: '备份记录', icon: <IconHistory /> },
  { key: '/restore/records', label: '恢复记录', icon: <IconRefresh /> },
  { key: '/verify/records', label: '验证演练', icon: <IconSafe /> },
  { key: '/replication/records', label: '备份复制', icon: <IconCopy /> },
  { key: '/task-templates', label: '任务模板', icon: <IconBook /> },
  { key: '/storage-targets', label: '存储目标', icon: <IconStorage /> },
  { key: '/nodes', label: '节点管理', icon: <IconDesktop /> },
  { key: '/settings/notifications', label: '通知配置', icon: <IconNotification /> },
  { key: '/admin/users', label: '用户管理', icon: <IconUser />, adminOnly: true },
  { key: '/admin/api-keys', label: 'API Key', icon: <IconCommand />, adminOnly: true },
  { key: '/audit', label: '审计日志', icon: <IconList /> },
  { key: '/settings', label: '系统设置', icon: <IconSettings /> },
]

export function AppLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const [pwdVisible, setPwdVisible] = useState(false)
  const [pwdLoading, setPwdLoading] = useState(false)
  const [twoFactorVisible, setTwoFactorVisible] = useState(false)
  const [twoFactorLoading, setTwoFactorLoading] = useState(false)
  const [twoFactorSetup, setTwoFactorSetup] = useState<TwoFactorSetupResult | null>(null)
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [webAuthnCredentials, setWebAuthnCredentials] = useState<WebAuthnCredential[]>([])
  const [trustedDevices, setTrustedDevices] = useState<TrustedDevice[]>([])
  const [securityDetailsLoading, setSecurityDetailsLoading] = useState(false)
  const [pwdForm] = Form.useForm<ChangePasswordPayload & { confirmPassword: string }>()
  const [twoFactorForm] = Form.useForm<{ currentPassword: string; code: string; email: string; phone: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)
  const setUser = useAuthStore((state) => state.setUser)

  function applySecurityUserUpdate(updated: UserInfo) {
    setUser(updated)
    if (!updated.mfaEnabled) {
      clearTrustedDeviceToken(updated.username)
    }
  }

  async function handleChangePassword() {
    try {
      const values = await pwdForm.validate()
      if (values.newPassword !== values.confirmPassword) {
        Message.error('两次输入的新密码不一致')
        return
      }
      setPwdLoading(true)
      await changePassword({ oldPassword: values.oldPassword, newPassword: values.newPassword })
      clearTrustedDeviceToken(user?.username)
      Message.success('密码修改成功')
      setPwdVisible(false)
      pwdForm.resetFields()
    } catch (err) {
      if (err) {
        Message.error(resolveErrorMessage(err, '密码修改失败'))
      }
    } finally {
      setPwdLoading(false)
    }
  }

  function closeTwoFactorModal() {
    setTwoFactorVisible(false)
    setTwoFactorSetup(null)
    setRecoveryCodes([])
    setWebAuthnCredentials([])
    setTrustedDevices([])
    twoFactorForm.resetFields()
  }

  async function openSecurityModal() {
    setTwoFactorVisible(true)
    twoFactorForm.setFieldValue('email', user?.email ?? '')
    twoFactorForm.setFieldValue('phone', user?.phone ?? '')
    await loadSecurityDetails()
  }

  async function loadSecurityDetails() {
    setSecurityDetailsLoading(true)
    try {
      const [credentials, devices] = await Promise.all([listWebAuthnCredentials(), listTrustedDevices()])
      setWebAuthnCredentials(credentials)
      setTrustedDevices(devices)
    } catch (err) {
      Message.error(resolveErrorMessage(err, '加载安全配置失败'))
    } finally {
      setSecurityDetailsLoading(false)
    }
  }

  async function copyRecoveryCodes() {
    if (recoveryCodes.length === 0) return
    try {
      await navigator.clipboard.writeText(recoveryCodes.join('\n'))
      Message.success('已复制到剪贴板')
    } catch {
      Message.info('请手动选择文本复制')
    }
  }

  async function handleTwoFactorSetupAction() {
    try {
      const values = await twoFactorForm.validate()
      setTwoFactorLoading(true)
      if (!twoFactorSetup) {
        const setup = await prepareTwoFactor({ currentPassword: values.currentPassword })
        setTwoFactorSetup(setup)
        Message.success('TOTP 密钥已生成')
        return
      }
      const result = await enableTwoFactor({ code: values.code })
      setUser(result.user)
      setRecoveryCodes(result.recoveryCodes)
      Message.success('TOTP 已启用')
    } catch (err) {
      if (err) {
        Message.error(resolveErrorMessage(err, 'TOTP 操作失败'))
      }
    } finally {
      setTwoFactorLoading(false)
    }
  }

  async function handleRegenerateRecoveryCodes() {
    try {
      const values = await twoFactorForm.validate()
      setTwoFactorLoading(true)
      const result = await regenerateRecoveryCodes({
        currentPassword: values.currentPassword,
        code: values.code,
      })
      setUser(result.user)
      setRecoveryCodes(result.recoveryCodes)
      twoFactorForm.resetFields()
      Message.success('恢复码已重新生成')
    } catch (err) {
      if (err) {
        Message.error(resolveErrorMessage(err, '恢复码生成失败'))
      }
    } finally {
      setTwoFactorLoading(false)
    }
  }

  async function handleDisableTwoFactor() {
    try {
      const values = await twoFactorForm.validate()
      setTwoFactorLoading(true)
      const updated = await disableTwoFactor({
        currentPassword: values.currentPassword,
        code: values.code,
      })
      applySecurityUserUpdate(updated)
      Message.success('TOTP 已关闭')
      closeTwoFactorModal()
    } catch (err) {
      if (err) {
        Message.error(resolveErrorMessage(err, '关闭 TOTP 失败'))
      }
    } finally {
      setTwoFactorLoading(false)
    }
  }

  function readCurrentPassword() {
    const currentPassword = String(twoFactorForm.getFieldValue('currentPassword') ?? '')
    if (currentPassword.trim().length < 8) {
      Message.error('请输入当前密码')
      return ''
    }
    return currentPassword
  }

  async function handleRegisterWebAuthn() {
    const currentPassword = readCurrentPassword()
    if (!currentPassword) return
    try {
      setTwoFactorLoading(true)
      const options = await beginWebAuthnRegistration({ currentPassword })
      const credential = await createWebAuthnCredential(options)
      const updated = await finishWebAuthnRegistration({ name: navigator.userAgent.slice(0, 120), credential })
      applySecurityUserUpdate(updated)
      await loadSecurityDetails()
      Message.success('通行密钥已注册')
    } catch (err) {
      Message.error(resolveErrorMessage(err, '通行密钥注册失败'))
    } finally {
      setTwoFactorLoading(false)
    }
  }

  async function handleDeleteWebAuthnCredential(id: string) {
    const currentPassword = readCurrentPassword()
    if (!currentPassword) return
    try {
      setTwoFactorLoading(true)
      const updated = await deleteWebAuthnCredential(id, { currentPassword })
      applySecurityUserUpdate(updated)
      await loadSecurityDetails()
      Message.success('通行密钥已删除')
    } catch (err) {
      Message.error(resolveErrorMessage(err, '删除通行密钥失败'))
    } finally {
      setTwoFactorLoading(false)
    }
  }

  async function handleConfigureOtp(channel: 'email' | 'sms', enabled: boolean) {
    const currentPassword = readCurrentPassword()
    if (!currentPassword) return
    const email = String(twoFactorForm.getFieldValue('email') ?? '')
    const phone = String(twoFactorForm.getFieldValue('phone') ?? '')
    try {
      setTwoFactorLoading(true)
      const updated = await configureOtp({ currentPassword, channel, enabled, email, phone })
      applySecurityUserUpdate(updated)
      twoFactorForm.setFieldValue('email', updated.email ?? '')
      twoFactorForm.setFieldValue('phone', updated.phone ?? '')
      Message.success(enabled ? 'OTP 已启用' : 'OTP 已关闭')
    } catch (err) {
      Message.error(resolveErrorMessage(err, 'OTP 配置失败'))
    } finally {
      setTwoFactorLoading(false)
    }
  }

  async function handleRevokeTrustedDevice(id: string) {
    const currentPassword = readCurrentPassword()
    if (!currentPassword) return
    try {
      setTwoFactorLoading(true)
      await revokeTrustedDevice(id, { currentPassword })
      clearTrustedDeviceToken(user?.username)
      await loadSecurityDetails()
      Message.success('可信设备已移除')
    } catch (err) {
      Message.error(resolveErrorMessage(err, '移除可信设备失败'))
    } finally {
      setTwoFactorLoading(false)
    }
  }

  function renderTwoFactorFooter() {
    if (recoveryCodes.length > 0) {
      return (
        <Space>
          <Button onClick={() => void copyRecoveryCodes()}>复制恢复码</Button>
          <Button type="primary" onClick={closeTwoFactorModal}>完成</Button>
        </Space>
      )
    }
    if (user?.twoFactorEnabled) {
      return (
        <Space>
          <Button onClick={closeTwoFactorModal}>取消</Button>
          <Button loading={twoFactorLoading} onClick={() => void handleRegenerateRecoveryCodes()}>重新生成恢复码</Button>
          <Button status="danger" loading={twoFactorLoading} onClick={() => void handleDisableTwoFactor()}>关闭 TOTP</Button>
        </Space>
      )
    }
    return (
      <Space>
        <Button onClick={closeTwoFactorModal}>取消</Button>
        <Button type="primary" loading={twoFactorLoading} onClick={() => void handleTwoFactorSetupAction()}>
          {twoFactorSetup ? '启用 TOTP' : '生成 TOTP 二维码'}
        </Button>
      </Space>
    )
  }

  const userDroplist = (
    <Menu onClickMenuItem={(key) => {
      if (key === 'password') {
        setPwdVisible(true)
      } else if (key === 'two-factor') {
        void openSecurityModal()
      } else if (key === 'logout') {
        logout()
      }
    }}>
      <Menu.Item key="password"><IconLock style={{ marginRight: 8 }} />修改密码</Menu.Item>
      <Menu.Item key="two-factor"><IconSafe style={{ marginRight: 8 }} />多因素认证</Menu.Item>
      <Menu.Item key="logout"><IconPoweroff style={{ marginRight: 8 }} />退出登录</Menu.Item>
    </Menu>
  )

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} trigger={null} breakpoint="lg" width={220}>
        <div style={{ padding: '20px 16px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <IconCloud style={{ fontSize: 28, color: 'var(--color-primary-6)' }} />
          {!collapsed && <Typography.Title heading={5} style={{ margin: 0, fontWeight: 700 }}>BackupX</Typography.Title>}
        </div>
        <Menu selectedKeys={[resolveSelectedKey(location.pathname)]} onClickMenuItem={(key) => navigate(key)}>
          {menuItems
            .filter((item) => !item.adminOnly || isAdmin(user))
            .map((item) => (
              <Menu.Item key={item.key}>
                {item.icon}
                {item.label}
              </Menu.Item>
            ))}
        </Menu>
        {!collapsed && (
          <div style={{ position: 'absolute', bottom: 16, left: 0, right: 0, textAlign: 'center' }}>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>v1.0.0</Typography.Text>
          </div>
        )}
      </Sider>
      <Layout>
        <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 20px', background: 'var(--color-bg-2)', borderBottom: '1px solid var(--color-border)' }}>
          <Space>
            <Button
              type="text"
              icon={collapsed ? <IconMenuUnfold /> : <IconMenuFold />}
              onClick={() => setCollapsed((value) => !value)}
            />
            <GlobalSearch />
          </Space>
          <Space>
            <EventCenter />
            <Dropdown droplist={userDroplist} position="br">
              <Button type="text" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Avatar size={28} style={{ backgroundColor: 'var(--color-primary-6)' }}>
                  {(user?.displayName ?? user?.username ?? '管')[0]}
                </Avatar>
                <span>{user?.displayName ?? user?.username ?? '管理员'}</span>
                <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>[{roleLabel(user?.role)}]</span>
                <IconDown />
              </Button>
            </Dropdown>
          </Space>
        </Header>
        <Content style={{ padding: '24px', background: 'var(--color-fill-2)', overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>

      <Modal
        title="修改密码"
        visible={pwdVisible}
        onCancel={() => { setPwdVisible(false); pwdForm.resetFields() }}
        onOk={handleChangePassword}
        confirmLoading={pwdLoading}
        unmountOnExit
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item field="oldPassword" label="当前密码" rules={[{ required: true, minLength: 8 }]}>
            <Input.Password placeholder="请输入当前密码" />
          </Form.Item>
          <Form.Item field="newPassword" label="新密码" rules={[{ required: true, minLength: 8 }]}>
            <Input.Password placeholder="请输入新密码（至少 8 位）" />
          </Form.Item>
          <Form.Item field="confirmPassword" label="确认新密码" rules={[{ required: true, minLength: 8 }]}>
            <Input.Password placeholder="请再次输入新密码" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="多因素认证"
        visible={twoFactorVisible}
        onCancel={closeTwoFactorModal}
        footer={renderTwoFactorFooter()}
        unmountOnExit
      >
        {recoveryCodes.length > 0 ? (
          <Space direction="vertical" size="medium" style={{ width: '100%' }}>
            <Alert type="warning" content="恢复码只会显示一次。请立即保存；每个恢复码只能使用一次。" />
            <Input.TextArea value={recoveryCodes.join('\n')} autoSize readOnly />
          </Space>
        ) : (
          <Form form={twoFactorForm} layout="vertical">
            {user?.twoFactorEnabled ? (
              <>
                <Alert
                  type="success"
                  content={`当前账号已启用 TOTP，恢复码剩余 ${user.twoFactorRecoveryCodesRemaining ?? 0} 个。`}
                  style={{ marginBottom: 16 }}
                />
                <Form.Item field="currentPassword" label="当前密码" rules={[{ required: true, minLength: 8 }]}>
                  <Input.Password placeholder="请输入当前密码" />
                </Form.Item>
                <Form.Item field="code" label="TOTP 验证码" rules={[{ required: true, minLength: 6, maxLength: 10 }]}>
                  <Input placeholder="请输入 6 位验证码" maxLength={10} />
                </Form.Item>
              </>
            ) : (
              <>
                {!twoFactorSetup ? (
                  <>
                    <Alert type="info" content="启用前需要验证当前密码。" style={{ marginBottom: 16 }} />
                    <Form.Item field="currentPassword" label="当前密码" rules={[{ required: true, minLength: 8 }]}>
                      <Input.Password placeholder="请输入当前密码" />
                    </Form.Item>
                  </>
                ) : (
                  <>
                    <Alert type="warning" content="密钥仅在本次启用流程中显示。启用后会生成一次性恢复码。" style={{ marginBottom: 16 }} />
                    <div style={{ display: 'flex', gap: 20, alignItems: 'center', marginBottom: 16 }}>
                      <img
                        src={twoFactorSetup.qrCodeDataUrl}
                        alt="TOTP 二维码"
                        style={{ width: 160, height: 160, border: '1px solid var(--color-border)', borderRadius: 8 }}
                      />
                      <Space direction="vertical" size={8} style={{ flex: 1, minWidth: 0 }}>
                        <Typography.Text type="secondary">手动密钥</Typography.Text>
                        <Input value={twoFactorSetup.secret} readOnly />
                      </Space>
                    </div>
                    <Form.Item field="code" label="TOTP 验证码" rules={[{ required: true, minLength: 6, maxLength: 10 }]}>
                      <Input placeholder="请输入 6 位验证码" maxLength={10} />
                    </Form.Item>
                  </>
                )}
              </>
            )}
            <Divider />
            <Space direction="vertical" size="medium" style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Title heading={6} style={{ margin: 0 }}>通行密钥</Typography.Title>
                <Tag color={webAuthnCredentials.length > 0 ? 'green' : 'gray'} bordered>
                  {webAuthnCredentials.length > 0 ? `${webAuthnCredentials.length} 个` : '未注册'}
                </Tag>
              </Space>
              <Typography.Paragraph type="secondary" style={{ margin: 0 }}>
                支持浏览器 Passkey、平台验证器或安全密钥，用于登录时替代验证码。
              </Typography.Paragraph>
              <Button loading={twoFactorLoading} onClick={() => void handleRegisterWebAuthn()}>注册当前设备通行密钥</Button>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                {securityDetailsLoading ? <Typography.Text type="secondary">正在加载通行密钥...</Typography.Text> : null}
                {webAuthnCredentials.map((item) => (
                  <div key={item.id} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', padding: '8px 0', borderTop: '1px solid var(--color-border)' }}>
                    <Space direction="vertical" size={2}>
                      <Typography.Text>{item.name}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>{item.lastUsedAt ? `最近使用 ${item.lastUsedAt}` : `创建于 ${item.createdAt}`}</Typography.Text>
                    </Space>
                    <Button size="small" status="danger" onClick={() => void handleDeleteWebAuthnCredential(item.id)}>删除</Button>
                  </div>
                ))}
              </Space>
            </Space>
            <Divider />
            <Space direction="vertical" size="medium" style={{ width: '100%' }}>
              <Typography.Title heading={6} style={{ margin: 0 }}>邮件 / 短信 OTP</Typography.Title>
              <Alert type="info" content="邮件 OTP 使用已启用的 Email 通知配置发送；短信 OTP 使用 SMS Webhook 通知配置发送。" />
              <Space wrap>
                <Tag color={user?.emailOtpEnabled ? 'green' : 'gray'} bordered>邮件 OTP {user?.emailOtpEnabled ? '已启用' : '未启用'}</Tag>
                <Tag color={user?.smsOtpEnabled ? 'green' : 'gray'} bordered>短信 OTP {user?.smsOtpEnabled ? '已启用' : '未启用'}</Tag>
              </Space>
              <Form.Item field="email" label="邮箱">
                <Input placeholder="启用邮件 OTP 时填写" />
              </Form.Item>
              <Form.Item field="phone" label="手机号">
                <Input placeholder="启用短信 OTP 时填写" />
              </Form.Item>
              <Space wrap>
                <Button loading={twoFactorLoading} onClick={() => void handleConfigureOtp('email', !user?.emailOtpEnabled)}>
                  {user?.emailOtpEnabled ? '关闭邮件 OTP' : '启用邮件 OTP'}
                </Button>
                <Button loading={twoFactorLoading} onClick={() => void handleConfigureOtp('sms', !user?.smsOtpEnabled)}>
                  {user?.smsOtpEnabled ? '关闭短信 OTP' : '启用短信 OTP'}
                </Button>
              </Space>
            </Space>
            <Divider />
            <Space direction="vertical" size="medium" style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Title heading={6} style={{ margin: 0 }}>可信设备</Typography.Title>
                <Tag color={trustedDevices.length > 0 ? 'green' : 'gray'} bordered>{trustedDevices.length} 个</Tag>
              </Space>
              <Typography.Paragraph type="secondary" style={{ margin: 0 }}>
                登录时勾选“信任此设备”后，30 天内该设备可在密码校验通过后跳过多因素验证。
              </Typography.Paragraph>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                {trustedDevices.map((item) => (
                  <div key={item.id} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', padding: '8px 0', borderTop: '1px solid var(--color-border)' }}>
                    <Space direction="vertical" size={2}>
                      <Typography.Text>{item.name}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>最近使用 {item.lastUsedAt || '-'}，到期 {item.expiresAt}</Typography.Text>
                    </Space>
                    <Button size="small" status="danger" onClick={() => void handleRevokeTrustedDevice(item.id)}>移除</Button>
                  </div>
                ))}
                {!securityDetailsLoading && trustedDevices.length === 0 ? <Typography.Text type="secondary">暂无可信设备</Typography.Text> : null}
              </Space>
            </Space>
          </Form>
        )}
      </Modal>
    </Layout>
  )
}
