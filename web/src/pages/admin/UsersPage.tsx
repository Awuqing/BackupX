import {
  Alert,
  Button,
  Empty,
  Form,
  Grid,
  Input,
  Message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@arco-design/web-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AdminDataSection } from '../../components/admin/AdminDataSection'
import { AdminRoleSelect, adminRoleDescriptions, adminRoleOptions } from '../../components/admin/AdminRoleSelect'
import { IconDelete, IconEdit, IconList, IconPlus, IconRefresh, IconSafe, IconSearch } from '../../components/icons'
import { clearTrustedDeviceToken } from '../../services/auth'
import {
  createUser,
  deleteUser,
  listUsers,
  resetUserTwoFactor,
  updateUser,
  type UserRole,
  type UserSummary,
  type UserUpsertPayload,
} from '../../services/users'
import { useAuthStore } from '../../stores/auth'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime } from '../../utils/format'
import { roleLabel } from '../../utils/permissions'

type UserStatusFilter = 'all' | 'enabled' | 'disabled'

function createEmpty(): UserUpsertPayload {
  return { username: '', password: '', displayName: '', email: '', phone: '', role: 'operator', disabled: false }
}

export function UsersPage() {
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const setUser = useAuthStore((state) => state.setUser)
  const [items, setItems] = useState<UserSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<UserRole | 'all'>('all')
  const [statusFilter, setStatusFilter] = useState<UserStatusFilter>('all')
  const [editing, setEditing] = useState<UserSummary | null>(null)
  const [modalVisible, setModalVisible] = useState(false)
  const [draft, setDraft] = useState<UserUpsertPayload>(createEmpty())
  const [submitting, setSubmitting] = useState(false)
  const [rowAction, setRowAction] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setItems(await listUsers())
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载用户失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const filteredItems = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return items.filter((item) => {
      const matchesQuery = !keyword || [item.username, item.displayName, item.email, item.phone]
        .some((value) => value?.toLowerCase().includes(keyword))
      const matchesRole = roleFilter === 'all' || item.role === roleFilter
      const matchesStatus = statusFilter === 'all'
        || (statusFilter === 'enabled' && !item.disabled)
        || (statusFilter === 'disabled' && item.disabled)
      return matchesQuery && matchesRole && matchesStatus
    })
  }, [items, query, roleFilter, statusFilter])

  const enabledCount = items.filter((item) => !item.disabled).length
  const adminCount = items.filter((item) => item.role === 'admin').length
  const mfaCount = items.filter((item) => !item.disabled && item.mfaEnabled).length
  const filtersActive = Boolean(query.trim()) || roleFilter !== 'all' || statusFilter !== 'all'

  function openCreate() {
    setEditing(null)
    setDraft(createEmpty())
    setModalVisible(true)
  }

  function openEdit(item: UserSummary) {
    setEditing(item)
    setDraft({
      username: item.username,
      password: '',
      displayName: item.displayName,
      email: item.email,
      phone: item.phone,
      role: item.role,
      disabled: item.disabled,
    })
    setModalVisible(true)
  }

  async function handleSubmit() {
    const payload: UserUpsertPayload = {
      ...draft,
      username: draft.username.trim(),
      displayName: draft.displayName.trim(),
      email: draft.email?.trim(),
      phone: draft.phone?.trim(),
    }
    if (payload.username.length < 3) {
      Message.error('用户名至少需要 3 个字符')
      return
    }
    if (!payload.displayName) {
      Message.error('显示名称不能为空')
      return
    }
    if ((!editing || payload.password?.trim()) && (payload.password?.length ?? 0) < 8) {
      Message.error(editing ? '新密码至少需要 8 个字符' : '初始密码至少需要 8 个字符')
      return
    }

    setSubmitting(true)
    try {
      if (editing) {
        const updated = await updateUser(editing.id, payload)
        if (updated.id === user?.id) {
          if (payload.password?.trim()) {
            clearTrustedDeviceToken(updated.username)
          }
          setUser(updated)
        }
        Message.success('用户已更新')
      } else {
        await createUser(payload)
        Message.success('用户已创建')
      }
      setModalVisible(false)
      await load()
    } catch (submitError) {
      Message.error(resolveErrorMessage(submitError, '保存失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(item: UserSummary) {
    setRowAction(`delete:${item.id}`)
    try {
      await deleteUser(item.id)
      Message.success('用户已删除')
      await load()
    } catch (deleteError) {
      Message.error(resolveErrorMessage(deleteError, '删除失败'))
    } finally {
      setRowAction('')
    }
  }

  async function handleResetTwoFactor(item: UserSummary) {
    setRowAction(`mfa:${item.id}`)
    try {
      const updated = await resetUserTwoFactor(item.id)
      if (updated.id === user?.id) {
        clearTrustedDeviceToken(updated.username)
        setUser(updated)
      }
      Message.success('MFA 已重置')
      await load()
    } catch (resetError) {
      Message.error(resolveErrorMessage(resetError, '重置 MFA 失败'))
    } finally {
      setRowAction('')
    }
  }

  const editingSelf = editing?.id === user?.id

  return (
    <AdminDataSection
      title="用户账号"
      description="维护登录账号、角色、联系方式和多因素认证状态。当前账号与最后一个管理员受到界面级保护。"
      metrics={[
        { label: '账号总数', value: items.length, detail: '系统中的全部登录账号' },
        { label: '启用账号', value: enabledCount, detail: `${items.length - enabledCount} 个账号已停用` },
        { label: '管理员', value: adminCount, detail: '拥有访问管理权限' },
        { label: 'MFA 覆盖', value: `${mfaCount}/${enabledCount}`, detail: '已启用账号中的 MFA 使用情况' },
      ]}
      actions={(
        <Space>
          <Button icon={<IconList />} onClick={() => navigate('/audit?category=user')}>用户审计</Button>
          <Button type="primary" icon={<IconPlus />} onClick={openCreate}>新建用户</Button>
        </Space>
      )}
      toolbar={(
        <div className="admin-toolbar">
          <div className="admin-toolbar__filters">
            <Input
              style={{ width: 260 }}
              allowClear
              prefix={<IconSearch />}
              value={query}
              aria-label="搜索用户"
              placeholder="搜索用户名、名称或联系方式"
              onChange={setQuery}
            />
            <Select
              style={{ width: 150 }}
              value={roleFilter}
              options={[{ label: '全部角色', value: 'all' }, ...adminRoleOptions]}
              onChange={(value) => setRoleFilter(value as UserRole | 'all')}
            />
            <Select
              style={{ width: 140 }}
              value={statusFilter}
              options={[
                { label: '全部状态', value: 'all' },
                { label: '已启用', value: 'enabled' },
                { label: '已停用', value: 'disabled' },
              ]}
              onChange={(value) => setStatusFilter(value as UserStatusFilter)}
            />
            <Button
              type="text"
              disabled={!filtersActive}
              onClick={() => { setQuery(''); setRoleFilter('all'); setStatusFilter('all') }}
            >
              清除筛选
            </Button>
          </div>
          <div className="admin-toolbar__status">
            <Typography.Text type="secondary">显示 {filteredItems.length} / {items.length}</Typography.Text>
            <Button icon={<IconRefresh />} loading={loading} onClick={() => void load()}>刷新</Button>
          </div>
        </div>
      )}
    >
      {error ? (
        <div className="admin-data-panel__alert">
          <Alert type="error" content={error} />
        </div>
      ) : null}
      <Table
        rowKey="id"
        loading={loading}
        data={filteredItems}
        stripe
        pagination={filteredItems.length > 10 ? { pageSize: 10 } : false}
        noDataElement={<Empty description={filtersActive ? '没有符合筛选条件的用户' : '暂无用户'} />}
        columns={[
          {
            title: '用户',
            dataIndex: 'username',
            width: 170,
            render: (value: string, row: UserSummary) => (
              <div className="admin-identity">
                <Space size={6}>
                  <Typography.Text>{value}</Typography.Text>
                  {row.id === user?.id ? <Tag bordered>当前账号</Tag> : null}
                </Space>
                <span className="admin-identity__secondary">{row.displayName}</span>
                <span className="admin-identity__secondary" title={formatDateTime(row.createdAt)}>
                  创建于 {formatDateTime(row.createdAt)}
                </span>
              </div>
            ),
          },
          {
            title: '角色',
            dataIndex: 'role',
            width: 80,
            render: (value: string) => <Tag color="arcoblue" bordered>{roleLabel(value)}</Tag>,
          },
          {
            title: '联系方式',
            dataIndex: 'email',
            width: 190,
            render: (_: string, row: UserSummary) => (
              <div className="admin-contact">
                <Typography.Text>{row.email || '未配置邮箱'}</Typography.Text>
                <span className="admin-contact__secondary">{row.phone || '未配置手机号'}</span>
              </div>
            ),
          },
          {
            title: '状态',
            dataIndex: 'disabled',
            width: 80,
            render: (disabled: boolean) => disabled
              ? <Tag color="red" bordered>已停用</Tag>
              : <Tag color="green" bordered>已启用</Tag>,
          },
          {
            title: '多因素认证',
            dataIndex: 'mfaEnabled',
            width: 210,
            render: (_: boolean, row: UserSummary) => row.mfaEnabled ? (
              <Space wrap size={4}>
                {row.twoFactorEnabled ? <Tag color="green" bordered>TOTP</Tag> : null}
                {row.webAuthnEnabled ? <Tag color="arcoblue" bordered>Passkey {row.webAuthnCredentialCount}</Tag> : null}
                {row.emailOtpEnabled ? <Tag color="purple" bordered>邮件</Tag> : null}
                {row.smsOtpEnabled ? <Tag color="orange" bordered>短信</Tag> : null}
                {row.trustedDeviceCount > 0 ? <Tag bordered>可信设备 {row.trustedDeviceCount}</Tag> : null}
                {row.twoFactorEnabled ? <Typography.Text type="secondary">恢复码 {row.twoFactorRecoveryCodesRemaining}</Typography.Text> : null}
              </Space>
            ) : <Tag bordered>未启用</Tag>,
          },
          {
            title: '操作',
            width: 270,
            render: (_: unknown, row: UserSummary) => {
              const deleteDisabled = row.id === user?.id || (row.role === 'admin' && adminCount <= 1)
              const deleteReason = row.id === user?.id ? '不能删除当前登录账号' : '不能删除系统最后一个管理员'
              return (
                <Space wrap>
                  <Button size="small" type="text" icon={<IconEdit />} onClick={() => openEdit(row)}>编辑</Button>
                  {row.mfaEnabled ? (
                    <Popconfirm
                      title={`确定重置用户「${row.username}」的全部 MFA 配置？`}
                      content="重置后，该用户可仅凭密码登录。"
                      onOk={() => handleResetTwoFactor(row)}
                    >
                      <Button
                        size="small"
                        type="text"
                        icon={<IconSafe />}
                        loading={rowAction === `mfa:${row.id}`}
                      >
                        重置 MFA
                      </Button>
                    </Popconfirm>
                  ) : null}
                  {deleteDisabled ? (
                    <Tooltip content={deleteReason}>
                      <span>
                        <Button size="small" type="text" status="danger" icon={<IconDelete />} disabled>删除</Button>
                      </span>
                    </Tooltip>
                  ) : (
                    <Popconfirm
                      title={`确定删除用户「${row.username}」？`}
                      content="删除后无法恢复。"
                      onOk={() => handleDelete(row)}
                    >
                      <Button
                        size="small"
                        type="text"
                        status="danger"
                        icon={<IconDelete />}
                        loading={rowAction === `delete:${row.id}`}
                      >
                        删除
                      </Button>
                    </Popconfirm>
                  )}
                </Space>
              )
            },
          },
        ]}
      />

      <Modal
        visible={modalVisible}
        title={editing ? '编辑用户' : '新建用户'}
        style={{ width: 680 }}
        onCancel={() => setModalVisible(false)}
        onOk={handleSubmit}
        confirmLoading={submitting}
        unmountOnExit
      >
        <Form layout="vertical">
          <Grid.Row gutter={16}>
            <Grid.Col span={12}>
              <Form.Item label="用户名" required>
                <Input
                  value={draft.username}
                  placeholder="至少 3 个字符"
                  disabled={Boolean(editing)}
                  onChange={(value) => setDraft({ ...draft, username: value })}
                />
              </Form.Item>
            </Grid.Col>
            <Grid.Col span={12}>
              <Form.Item label="显示名称" required>
                <Input value={draft.displayName} onChange={(value) => setDraft({ ...draft, displayName: value })} />
              </Form.Item>
            </Grid.Col>
          </Grid.Row>
          <Grid.Row gutter={16}>
            <Grid.Col span={12}>
              <Form.Item label="邮箱">
                <Input value={draft.email ?? ''} onChange={(value) => setDraft({ ...draft, email: value })} />
              </Form.Item>
            </Grid.Col>
            <Grid.Col span={12}>
              <Form.Item label="手机号">
                <Input value={draft.phone ?? ''} onChange={(value) => setDraft({ ...draft, phone: value })} />
              </Form.Item>
            </Grid.Col>
          </Grid.Row>
          <Form.Item label={editing ? '新密码（留空不修改）' : '初始密码'} required={!editing}>
            <Input.Password
              value={draft.password}
              placeholder="至少 8 个字符"
              onChange={(value) => setDraft({ ...draft, password: value })}
            />
          </Form.Item>
          <Grid.Row gutter={16}>
            <Grid.Col span={12}>
              <Form.Item label="角色" required>
                <AdminRoleSelect
                  value={draft.role}
                  disabled={editingSelf}
                  onChange={(role) => setDraft({ ...draft, role })}
                />
                <span className="admin-form-note">
                  {editingSelf ? '当前登录账号不能在此修改自身角色。' : adminRoleDescriptions[draft.role]}
                </span>
              </Form.Item>
            </Grid.Col>
            <Grid.Col span={12}>
              <Form.Item label="账号状态">
                <div className="admin-switch-field">
                  <Switch
                    checked={!draft.disabled}
                    disabled={editingSelf}
                    onChange={(enabled) => setDraft({ ...draft, disabled: !enabled })}
                  />
                  <Typography.Text>{draft.disabled ? '已停用' : '已启用'}</Typography.Text>
                </div>
                {editingSelf ? <span className="admin-form-note">当前登录账号不能停用自身。</span> : null}
              </Form.Item>
            </Grid.Col>
          </Grid.Row>
        </Form>
      </Modal>
    </AdminDataSection>
  )
}
