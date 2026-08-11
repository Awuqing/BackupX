import {
  Alert,
  Button,
  Empty,
  Form,
  Grid,
  Input,
  InputNumber,
  Message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@arco-design/web-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AdminDataSection } from '../../components/admin/AdminDataSection'
import { AdminRoleSelect, adminRoleDescriptions, adminRoleOptions } from '../../components/admin/AdminRoleSelect'
import { IconCopy, IconDelete, IconList, IconPlus, IconRefresh, IconSearch } from '../../components/icons'
import {
  createApiKey,
  listApiKeys,
  revokeApiKey,
  toggleApiKey,
  type ApiKeyCreateInput,
  type ApiKeySummary,
} from '../../services/api-keys'
import type { UserRole } from '../../services/users'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime } from '../../utils/format'
import { roleLabel } from '../../utils/permissions'

type ApiKeyStatus = 'active' | 'disabled' | 'expired'
type ApiKeyStatusFilter = ApiKeyStatus | 'all'

export function resolveApiKeyStatus(item: ApiKeySummary, now: number): ApiKeyStatus {
  if (item.expiresAt && new Date(item.expiresAt).getTime() <= now) {
    return 'expired'
  }
  return item.disabled ? 'disabled' : 'active'
}

export function ApiKeysPage() {
  const navigate = useNavigate()
  const [items, setItems] = useState<ApiKeySummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<UserRole | 'all'>('all')
  const [statusFilter, setStatusFilter] = useState<ApiKeyStatusFilter>('all')
  const [modalVisible, setModalVisible] = useState(false)
  const [draft, setDraft] = useState<ApiKeyCreateInput>({ name: '', role: 'viewer', ttlHours: 0 })
  const [submitting, setSubmitting] = useState(false)
  const [rowAction, setRowAction] = useState('')
  const [plainKey, setPlainKey] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setItems(await listApiKeys())
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载 API Key 失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const now = useMemo(() => Date.now(), [items])
  const filteredItems = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return items.filter((item) => {
      const matchesQuery = !keyword || [item.name, item.prefix, item.createdBy]
        .some((value) => value?.toLowerCase().includes(keyword))
      const matchesRole = roleFilter === 'all' || item.role === roleFilter
      const matchesStatus = statusFilter === 'all' || resolveApiKeyStatus(item, now) === statusFilter
      return matchesQuery && matchesRole && matchesStatus
    })
  }, [items, now, query, roleFilter, statusFilter])

  const activeCount = items.filter((item) => resolveApiKeyStatus(item, now) === 'active').length
  const disabledCount = items.filter((item) => resolveApiKeyStatus(item, now) === 'disabled').length
  const expiredCount = items.filter((item) => resolveApiKeyStatus(item, now) === 'expired').length
  const usedCount = items.filter((item) => Boolean(item.lastUsedAt)).length
  const filtersActive = Boolean(query.trim()) || roleFilter !== 'all' || statusFilter !== 'all'

  function openCreate() {
    setDraft({ name: '', role: 'viewer', ttlHours: 0 })
    setPlainKey('')
    setModalVisible(true)
  }

  function closeModal() {
    setModalVisible(false)
    setPlainKey('')
  }

  async function handleSubmit() {
    const payload = { ...draft, name: draft.name.trim(), ttlHours: Number(draft.ttlHours ?? 0) }
    if (!payload.name) {
      Message.error('名称不能为空')
      return
    }
    if (payload.ttlHours < 0 || payload.ttlHours > 87600) {
      Message.error('有效期需要在 0 到 87600 小时之间')
      return
    }

    setSubmitting(true)
    try {
      const result = await createApiKey(payload)
      setPlainKey(result.plainKey)
      await load()
    } catch (submitError) {
      Message.error(resolveErrorMessage(submitError, '创建失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleToggle(item: ApiKeySummary) {
    setRowAction(`toggle:${item.id}`)
    try {
      await toggleApiKey(item.id, !item.disabled)
      Message.success(item.disabled ? 'API Key 已启用' : 'API Key 已停用')
      await load()
    } catch (toggleError) {
      Message.error(resolveErrorMessage(toggleError, '操作失败'))
    } finally {
      setRowAction('')
    }
  }

  async function handleRevoke(item: ApiKeySummary) {
    setRowAction(`revoke:${item.id}`)
    try {
      await revokeApiKey(item.id)
      Message.success('API Key 已撤销')
      await load()
    } catch (revokeError) {
      Message.error(resolveErrorMessage(revokeError, '撤销失败'))
    } finally {
      setRowAction('')
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

  return (
    <AdminDataSection
      title="API Key"
      description="为 CI/CD、监控和自动化任务签发独立凭据，并集中管理权限、有效期、使用状态与撤销操作。"
      metrics={[
        { label: '凭据总数', value: items.length, detail: `${usedCount} 个凭据已有调用记录` },
        { label: '当前可用', value: activeCount, detail: '未停用且未超过有效期' },
        { label: '已停用', value: disabledCount, detail: '保留记录，可再次启用' },
        { label: '已过期', value: expiredCount, detail: '到期后无法继续认证' },
      ]}
      actions={(
        <Space>
          <Button icon={<IconList />} onClick={() => navigate('/audit?category=api_key')}>密钥审计</Button>
          <Button type="primary" icon={<IconPlus />} onClick={openCreate}>生成 API Key</Button>
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
              aria-label="搜索 API Key"
              placeholder="搜索名称、前缀或创建者"
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
                { label: '当前可用', value: 'active' },
                { label: '已停用', value: 'disabled' },
                { label: '已过期', value: 'expired' },
              ]}
              onChange={(value) => setStatusFilter(value as ApiKeyStatusFilter)}
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
        noDataElement={<Empty description={filtersActive ? '没有符合筛选条件的 API Key' : '暂无 API Key'} />}
        columns={[
          {
            title: '名称',
            dataIndex: 'name',
            width: 190,
            render: (value: string, row: ApiKeySummary) => (
              <div className="admin-identity">
                <Typography.Text>{value}</Typography.Text>
                <span className="admin-identity__secondary" title={`由 ${row.createdBy || '-'} 创建于 ${formatDateTime(row.createdAt)}`}>
                  由 {row.createdBy || '-'} 创建 · {formatDateTime(row.createdAt)}
                </span>
              </div>
            ),
          },
          {
            title: '角色',
            dataIndex: 'role',
            width: 90,
            render: (value: string) => <Tag color="arcoblue" bordered>{roleLabel(value)}</Tag>,
          },
          {
            title: 'Key 前缀',
            dataIndex: 'prefix',
            width: 135,
            render: (value: string) => <span className="admin-key-prefix">{value}…</span>,
          },
          {
            title: '最近使用',
            dataIndex: 'lastUsedAt',
            width: 160,
            render: (value?: string) => value ? <span className="admin-date">{formatDateTime(value)}</span> : '从未使用',
          },
          {
            title: '有效期',
            dataIndex: 'expiresAt',
            width: 160,
            render: (value?: string) => value ? <span className="admin-date">{formatDateTime(value)}</span> : '永不过期',
          },
          {
            title: '状态',
            dataIndex: 'disabled',
            width: 90,
            render: (_: boolean, row: ApiKeySummary) => {
              const status = resolveApiKeyStatus(row, now)
              if (status === 'expired') return <Tag color="orange" bordered>已过期</Tag>
              if (status === 'disabled') return <Tag color="red" bordered>已停用</Tag>
              return <Tag color="green" bordered>当前可用</Tag>
            },
          },
          {
            title: '操作',
            width: 150,
            render: (_: unknown, row: ApiKeySummary) => {
              const expired = resolveApiKeyStatus(row, now) === 'expired'
              return (
                <Space>
                  {expired ? (
                    <Tooltip content="已过期凭据不能重新启用，请生成新凭据">
                      <span><Button size="small" type="text" disabled>启用</Button></span>
                    </Tooltip>
                  ) : (
                    <Button
                      size="small"
                      type="text"
                      loading={rowAction === `toggle:${row.id}`}
                      onClick={() => void handleToggle(row)}
                    >
                      {row.disabled ? '启用' : '停用'}
                    </Button>
                  )}
                  <Popconfirm
                    title={`确定撤销 API Key「${row.name}」？`}
                    content="撤销后无法恢复，使用该凭据的自动化任务将立即失效。"
                    onOk={() => handleRevoke(row)}
                  >
                    <Button
                      size="small"
                      type="text"
                      status="danger"
                      icon={<IconDelete />}
                      loading={rowAction === `revoke:${row.id}`}
                    >
                      撤销
                    </Button>
                  </Popconfirm>
                </Space>
              )
            },
          },
        ]}
      />

      <Modal
        visible={modalVisible}
        title={plainKey ? '保存 API Key' : '生成 API Key'}
        style={{ width: 640 }}
        onCancel={closeModal}
        onOk={plainKey ? closeModal : handleSubmit}
        okText={plainKey ? '我已保存' : '生成'}
        confirmLoading={submitting}
        unmountOnExit
      >
        {plainKey ? (
          <div className="admin-key-result">
            <Alert type="warning" content="明文 Key 只显示一次。关闭窗口前，请将它保存到安全的密钥管理系统。" />
            <Input.TextArea value={plainKey} autoSize={{ minRows: 2, maxRows: 3 }} readOnly />
            <Button type="outline" icon={<IconCopy />} onClick={() => void copyPlainKey()}>复制到剪贴板</Button>
          </div>
        ) : (
          <Form layout="vertical">
            <Form.Item label="名称" required>
              <Input
                value={draft.name}
                maxLength={128}
                showWordLimit
                placeholder="例如：ci-deploy-script"
                onChange={(value) => setDraft({ ...draft, name: value })}
              />
            </Form.Item>
            <Grid.Row gutter={16}>
              <Grid.Col span={12}>
                <Form.Item label="角色" required>
                  <AdminRoleSelect value={draft.role} onChange={(role) => setDraft({ ...draft, role })} />
                  <span className="admin-form-note">{adminRoleDescriptions[draft.role]}</span>
                </Form.Item>
              </Grid.Col>
              <Grid.Col span={12}>
                <Form.Item label="有效期">
                  <InputNumber
                    style={{ width: '100%' }}
                    min={0}
                    max={87600}
                    suffix="小时"
                    value={draft.ttlHours ?? 0}
                    onChange={(value) => setDraft({ ...draft, ttlHours: Number(value ?? 0) })}
                  />
                  <span className="admin-form-note">0 表示永不过期；自动化凭据建议设置明确有效期。</span>
                </Form.Item>
              </Grid.Col>
            </Grid.Row>
          </Form>
        )}
      </Modal>
    </AdminDataSection>
  )
}
