import type { UserInfo } from '../services/auth'

// 用户角色常量，与后端 model.UserRole* 保持一致。
export const UserRole = {
  Admin: 'admin',
  Operator: 'operator',
  Viewer: 'viewer',
} as const

export type UserRoleType = typeof UserRole[keyof typeof UserRole]

/** 是否管理员角色。 */
export function isAdmin(user?: UserInfo | null): boolean {
  return (user?.role ?? '').toLowerCase() === UserRole.Admin
}

/** 是否只读（viewer）。 */
export function isViewer(user?: UserInfo | null): boolean {
  return (user?.role ?? '').toLowerCase() === UserRole.Viewer
}

/** 是否允许写入/变更类操作（admin 或 operator）。 */
export function canWrite(user?: UserInfo | null): boolean {
  const role = (user?.role ?? '').toLowerCase()
  return role === UserRole.Admin || role === UserRole.Operator
}

/** 角色展示名（用于 UI）。 */
export function roleLabel(role?: string): string {
  switch ((role ?? '').toLowerCase()) {
    case UserRole.Admin:
      return '管理员'
    case UserRole.Operator:
      return '运维'
    case UserRole.Viewer:
      return '只读'
    default:
      return role ?? '-'
  }
}
