import { Select } from '@arco-design/web-react'
import type { CSSProperties } from 'react'
import type { UserRole } from '../../services/users'

export const adminRoleOptions = [
  { label: '管理员 (admin)', value: 'admin' },
  { label: '运维 (operator)', value: 'operator' },
  { label: '只读 (viewer)', value: 'viewer' },
]

export const adminRoleDescriptions: Record<UserRole, string> = {
  admin: '拥有系统配置、账号与访问凭据的完整管理权限。',
  operator: '可执行日常备份、恢复和节点运维操作。',
  viewer: '仅可查看仪表盘和允许读取的数据。',
}

interface AdminRoleSelectProps {
  value: UserRole
  onChange: (role: UserRole) => void
  disabled?: boolean
  style?: CSSProperties
}

export function AdminRoleSelect({ value, onChange, disabled, style }: AdminRoleSelectProps) {
  return (
    <Select
      value={value}
      options={adminRoleOptions}
      disabled={disabled}
      style={style}
      onChange={(role) => onChange(role as UserRole)}
    />
  )
}
