import { Alert, Button, Card, Empty, Form, Input, Message, Modal, Select, Space, Switch, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { createUser, deleteUser, listUsers, resetUserTwoFactor, updateUser, type UserRole, type UserSummary, type UserUpsertPayload } from '../../services/users'
import { clearTrustedDeviceToken } from '../../services/auth'
import { useAuthStore } from '../../stores/auth'
import { resolveErrorMessage } from '../../utils/error'
import { isAdmin, roleLabel } from '../../utils/permissions'

const roleOptions = [
  { label: '管理员 (admin)', value: 'admin' },
  { label: '运维 (operator)', value: 'operator' },
  { label: '只读 (viewer)', value: 'viewer' },
]

function createEmpty(): UserUpsertPayload {
  return { username: '', password: '', displayName: '', email: '', phone: '', role: 'operator', disabled: false }
}

// UsersPage admin 用户管理。非 admin 角色进入路由会被路由守卫拦截。
export function UsersPage() {
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const [items, setItems] = useState<UserSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<UserSummary | null>(null)
  const [modalVisible, setModalVisible] = useState(false)
  const [draft, setDraft] = useState<UserUpsertPayload>(createEmpty())
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setItems(await listUsers())
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载用户失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

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
    if (!draft.username.trim() || !draft.displayName.trim()) {
      Message.error('用户名与显示名称不能为空')
      return
    }
    if (!editing && !draft.password?.trim()) {
      Message.error('创建用户必须设置初始密码')
      return
    }
    setSubmitting(true)
    try {
      if (editing) {
        const updated = await updateUser(editing.id, draft)
        if (updated.id === user?.id) {
          if (draft.password?.trim()) {
            clearTrustedDeviceToken(updated.username)
          }
          setUser(updated)
        }
        Message.success('用户已更新')
      } else {
        await createUser(draft)
        Message.success('用户已创建')
      }
      setModalVisible(false)
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '保存失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(item: UserSummary) {
    if (!window.confirm(`确定删除用户「${item.username}」吗？`)) return
    try {
      await deleteUser(item.id)
      Message.success('已删除')
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '删除失败'))
    }
  }

  async function handleResetTwoFactor(item: UserSummary) {
    if (!window.confirm(`确定重置用户「${item.username}」的全部 MFA 配置吗？该用户之后可仅凭密码登录。`)) return
    try {
      const updated = await resetUserTwoFactor(item.id)
      if (updated.id === user?.id) {
        clearTrustedDeviceToken(updated.username)
        setUser(updated)
      }
      Message.success('MFA 已重置')
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '重置 MFA 失败'))
    }
  }

  if (!isAdmin(user)) {
    return <Alert type="warning" content="当前账号无权访问用户管理（仅 admin）" />
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>用户管理</Typography.Title>
        <Typography.Paragraph type="secondary">管理系统账号。角色分为管理员（全权）、运维（日常运维）、只读（仪表盘）。</Typography.Paragraph>
      </div>

      <Space>
        <Button type="primary" onClick={openCreate}>新建用户</Button>
      </Space>

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}

      <Card>
        <Table
          rowKey="id"
          loading={loading}
          data={items}
          pagination={false}
          stripe
          noDataElement={<Empty description="暂无用户" />}
          columns={[
            { title: '用户名', dataIndex: 'username', render: (value: string, row: UserSummary) => (
              <Space direction="vertical" size={2}>
                <Typography.Text bold>{value}</Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{row.displayName}</Typography.Text>
              </Space>
            ) },
            { title: '角色', dataIndex: 'role', render: (value: string) => <Tag color="arcoblue" bordered>{roleLabel(value)}</Tag> },
            { title: '邮箱 / 手机', dataIndex: 'email', render: (_: string, row: UserSummary) => (
              <Space direction="vertical" size={2}>
                <Typography.Text>{row.email || '-'}</Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>{row.phone || '-'}</Typography.Text>
              </Space>
            ) },
            { title: '状态', dataIndex: 'disabled', render: (disabled: boolean) => disabled ? <Tag color="red" bordered>已停用</Tag> : <Tag color="green" bordered>启用</Tag> },
            { title: 'MFA', dataIndex: 'mfaEnabled', render: (_: boolean, row: UserSummary) => row.mfaEnabled ? (
              <Space wrap size={4}>
                {row.twoFactorEnabled ? <Tag color="green" bordered>TOTP</Tag> : null}
                {row.webAuthnEnabled ? <Tag color="arcoblue" bordered>Passkey {row.webAuthnCredentialCount}</Tag> : null}
                {row.emailOtpEnabled ? <Tag color="purple" bordered>邮件</Tag> : null}
                {row.smsOtpEnabled ? <Tag color="orange" bordered>短信</Tag> : null}
                {row.twoFactorEnabled ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>恢复码 {row.twoFactorRecoveryCodesRemaining}</Typography.Text> : null}
              </Space>
            ) : <Tag bordered>未启用</Tag> },
            { title: '创建时间', dataIndex: 'createdAt' },
            { title: '操作', width: 260, render: (_: unknown, row: UserSummary) => (
              <Space>
                <Button size="small" type="text" onClick={() => openEdit(row)}>编辑</Button>
                {row.mfaEnabled && <Button size="small" type="text" onClick={() => void handleResetTwoFactor(row)}>重置 MFA</Button>}
                <Button size="small" type="text" status="danger" onClick={() => void handleDelete(row)} disabled={row.id === user?.id}>删除</Button>
              </Space>
            ) },
          ]}
        />
      </Card>

      <Modal
        visible={modalVisible}
        title={editing ? '编辑用户' : '新建用户'}
        onCancel={() => setModalVisible(false)}
        onOk={handleSubmit}
        confirmLoading={submitting}
        unmountOnExit
      >
        <Form layout="vertical">
          <Form.Item label="用户名" required>
            <Input value={draft.username} onChange={(v) => setDraft({ ...draft, username: v })} disabled={!!editing} />
          </Form.Item>
          <Form.Item label="显示名称" required>
            <Input value={draft.displayName} onChange={(v) => setDraft({ ...draft, displayName: v })} />
          </Form.Item>
          <Form.Item label="邮箱">
            <Input value={draft.email} onChange={(v) => setDraft({ ...draft, email: v })} />
          </Form.Item>
          <Form.Item label="手机号">
            <Input value={draft.phone} onChange={(v) => setDraft({ ...draft, phone: v })} />
          </Form.Item>
          <Form.Item label={editing ? '新密码（留空不修改）' : '初始密码'} required={!editing}>
            <Input.Password value={draft.password} onChange={(v) => setDraft({ ...draft, password: v })} />
          </Form.Item>
          <Form.Item label="角色" required>
            <Select value={draft.role} options={roleOptions} onChange={(v: UserRole) => setDraft({ ...draft, role: v })} />
          </Form.Item>
          <Form.Item label="停用账号">
            <Switch checked={draft.disabled} onChange={(v) => setDraft({ ...draft, disabled: v })} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
