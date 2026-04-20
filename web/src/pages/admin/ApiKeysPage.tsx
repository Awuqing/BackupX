import { Alert, Button, Card, Empty, Form, Input, InputNumber, Message, Modal, Select, Space, Switch, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { createApiKey, listApiKeys, revokeApiKey, toggleApiKey, type ApiKeyCreateInput, type ApiKeySummary } from '../../services/api-keys'
import type { UserRole } from '../../services/users'
import { useAuthStore } from '../../stores/auth'
import { resolveErrorMessage } from '../../utils/error'
import { isAdmin, roleLabel } from '../../utils/permissions'
import { formatDateTime } from '../../utils/format'

const roleOptions = [
  { label: '管理员 (admin)', value: 'admin' },
  { label: '运维 (operator)', value: 'operator' },
  { label: '只读 (viewer)', value: 'viewer' },
]

// ApiKeysPage API Key 管理（admin 专属）。
// 新创建的 Key 明文只返回一次，需要用户立即保存。
export function ApiKeysPage() {
  const user = useAuthStore((s) => s.user)
  const [items, setItems] = useState<ApiKeySummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [modalVisible, setModalVisible] = useState(false)
  const [draft, setDraft] = useState<ApiKeyCreateInput>({ name: '', role: 'viewer', ttlHours: 0 })
  const [submitting, setSubmitting] = useState(false)
  const [plainKey, setPlainKey] = useState<string>('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setItems(await listApiKeys())
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载 API Key 失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  function openCreate() {
    setDraft({ name: '', role: 'viewer', ttlHours: 0 })
    setPlainKey('')
    setModalVisible(true)
  }

  async function handleSubmit() {
    if (!draft.name.trim()) {
      Message.error('名称不能为空')
      return
    }
    setSubmitting(true)
    try {
      const result = await createApiKey(draft)
      setPlainKey(result.plainKey)
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '创建失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleToggle(item: ApiKeySummary) {
    try {
      await toggleApiKey(item.id, !item.disabled)
      Message.success(item.disabled ? '已启用' : '已停用')
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '操作失败'))
    }
  }

  async function handleRevoke(item: ApiKeySummary) {
    if (!window.confirm(`确定撤销 API Key「${item.name}」？操作不可撤销。`)) return
    try {
      await revokeApiKey(item.id)
      Message.success('已撤销')
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '撤销失败'))
    }
  }

  async function copyPlainKey() {
    if (!plainKey) return
    try {
      await navigator.clipboard.writeText(plainKey)
      Message.success('已复制到剪贴板')
    } catch {
      Message.info('请手动选择文本复制')
    }
  }

  if (!isAdmin(user)) {
    return <Alert type="warning" content="当前账号无权访问 API Key 管理（仅 admin）" />
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>API Key</Typography.Title>
        <Typography.Paragraph type="secondary">
          签发 API Key 供 CI/CD、监控脚本等非交互式场景访问 BackupX。在请求头加 <Typography.Text code>Authorization: Bearer bax_xxx</Typography.Text> 或 <Typography.Text code>X-Api-Key: bax_xxx</Typography.Text> 即可。
        </Typography.Paragraph>
      </div>

      <Space>
        <Button type="primary" onClick={openCreate}>生成 API Key</Button>
      </Space>

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}

      <Card>
        <Table
          rowKey="id"
          loading={loading}
          data={items}
          pagination={false}
          stripe
          noDataElement={<Empty description="暂无 API Key" />}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '角色', dataIndex: 'role', render: (v: string) => <Tag color="arcoblue" bordered>{roleLabel(v)}</Tag> },
            { title: 'Key 前缀', dataIndex: 'prefix', render: (v: string) => <Typography.Text code>{v}…</Typography.Text> },
            { title: '创建者', dataIndex: 'createdBy', render: (v: string) => v || '-' },
            { title: '最近使用', dataIndex: 'lastUsedAt', render: (v?: string) => v ? formatDateTime(v) : '从未使用' },
            { title: '过期', dataIndex: 'expiresAt', render: (v?: string) => v ? formatDateTime(v) : '永不过期' },
            { title: '状态', dataIndex: 'disabled', render: (disabled: boolean) => disabled ? <Tag color="red" bordered>已停用</Tag> : <Tag color="green" bordered>启用</Tag> },
            { title: '操作', width: 180, render: (_: unknown, row: ApiKeySummary) => (
              <Space>
                <Button size="small" type="text" onClick={() => void handleToggle(row)}>{row.disabled ? '启用' : '停用'}</Button>
                <Button size="small" type="text" status="danger" onClick={() => void handleRevoke(row)}>撤销</Button>
              </Space>
            ) },
          ]}
        />
      </Card>

      <Modal
        visible={modalVisible}
        title="生成 API Key"
        onCancel={() => { setModalVisible(false); setPlainKey('') }}
        onOk={plainKey ? () => { setModalVisible(false); setPlainKey('') } : handleSubmit}
        okText={plainKey ? '完成' : '生成'}
        confirmLoading={submitting}
        unmountOnExit
      >
        {plainKey ? (
          <Space direction="vertical" size="medium" style={{ width: '100%' }}>
            <Alert type="warning" content="明文 Key 只会显示一次，请立即妥善保存。" />
            <Input.TextArea value={plainKey} autoSize readOnly />
            <Button type="outline" onClick={() => void copyPlainKey()}>复制到剪贴板</Button>
          </Space>
        ) : (
          <Form layout="vertical">
            <Form.Item label="名称" required>
              <Input value={draft.name} onChange={(v) => setDraft({ ...draft, name: v })} placeholder="例如：ci-deploy-script" />
            </Form.Item>
            <Form.Item label="角色" required>
              <Select value={draft.role} options={roleOptions} onChange={(v: UserRole) => setDraft({ ...draft, role: v })} />
            </Form.Item>
            <Form.Item label="有效期（小时，0=永不过期）">
              <InputNumber style={{ width: '100%' }} min={0} value={draft.ttlHours ?? 0} onChange={(v) => setDraft({ ...draft, ttlHours: Number(v ?? 0) })} />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </Space>
  )
}

// 避免未使用告警
void Switch
