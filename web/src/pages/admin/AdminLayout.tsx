import { Alert, Button, PageHeader } from '@arco-design/web-react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { IconCommand, IconList, IconUser } from '../../components/icons'
import { useAuthStore } from '../../stores/auth'
import { isAdmin } from '../../utils/permissions'
import './admin.css'

const sections = [
  { path: '/admin/users', label: '用户账号', icon: <IconUser /> },
  { path: '/admin/api-keys', label: 'API Key', icon: <IconCommand /> },
]

export function AdminLayout() {
  const user = useAuthStore((state) => state.user)
  const location = useLocation()
  const navigate = useNavigate()

  if (!isAdmin(user)) {
    return <Alert type="warning" content="当前账号无权进入访问管理（仅管理员）" />
  }

  return (
    <div className="admin-page">
      <PageHeader
        className="admin-page__header"
        title="访问管理"
        subTitle="统一管理系统账号、角色权限、多因素认证与程序化访问凭据。"
        extra={(
          <Button icon={<IconList />} onClick={() => navigate('/audit')}>
            访问审计
          </Button>
        )}
      />

      <nav className="admin-page__nav" aria-label="访问管理分区">
        {sections.map((section) => {
          const selected = location.pathname.startsWith(section.path)
          return (
            <Button
              key={section.path}
              type={selected ? 'secondary' : 'text'}
              icon={section.icon}
              aria-current={selected ? 'page' : undefined}
              onClick={() => navigate(section.path)}
            >
              {section.label}
            </Button>
          )
        })}
      </nav>

      <Outlet />
    </div>
  )
}
