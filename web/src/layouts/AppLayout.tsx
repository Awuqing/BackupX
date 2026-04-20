import { Avatar, Button, Dropdown, Layout, Menu, Message, Modal, Form, Input, Space, Typography } from '@arco-design/web-react'
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
import { changePassword, type ChangePasswordPayload } from '../services/auth'
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
  const [pwdForm] = Form.useForm<ChangePasswordPayload & { confirmPassword: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)

  async function handleChangePassword() {
    try {
      const values = await pwdForm.validate()
      if (values.newPassword !== values.confirmPassword) {
        Message.error('两次输入的新密码不一致')
        return
      }
      setPwdLoading(true)
      await changePassword({ oldPassword: values.oldPassword, newPassword: values.newPassword })
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

  const userDroplist = (
    <Menu onClickMenuItem={(key) => {
      if (key === 'password') {
        setPwdVisible(true)
      } else if (key === 'logout') {
        logout()
      }
    }}>
      <Menu.Item key="password"><IconLock style={{ marginRight: 8 }} />修改密码</Menu.Item>
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
    </Layout>
  )
}
