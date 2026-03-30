import { PageHeader, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import type { ColumnProps } from '@arco-design/web-react/es/Table'
import { useCallback, useEffect, useState } from 'react'
import { listAuditLogs } from '../../services/audit'
import type { AuditLog } from '../../types/audit'
import { formatDateTime } from '../../utils/format'
import { resolveErrorMessage } from '../../utils/error'

const categoryOptions = [
  { label: '全部', value: '' },
  { label: '认证', value: 'auth' },
  { label: '存储目标', value: 'storage_target' },
  { label: '备份任务', value: 'backup_task' },
  { label: '备份记录', value: 'backup_record' },
  { label: '系统设置', value: 'settings' },
]

const categoryLabels: Record<string, string> = {
  auth: '认证',
  storage_target: '存储目标',
  backup_task: '备份任务',
  backup_record: '备份记录',
  settings: '系统设置',
}

const actionLabels: Record<string, string> = {
  login_success: '登录成功',
  login_failed: '登录失败',
  setup: '系统初始化',
  change_password: '修改密码',
  create: '创建',
  update: '更新',
  delete: '删除',
  enable: '启用',
  disable: '停用',
  run: '执行',
  restore: '恢复',
}

const PAGE_SIZE = 20

const columns: ColumnProps<AuditLog>[] = [
  {
    title: '时间',
    dataIndex: 'createdAt',
    width: 180,
    render: (_, record) => formatDateTime(record.createdAt),
  },
  {
    title: '分类',
    dataIndex: 'category',
    width: 100,
    render: (_, record) => (
      <Tag bordered>{categoryLabels[record.category] ?? record.category}</Tag>
    ),
  },
  {
    title: '操作',
    dataIndex: 'action',
    width: 100,
    render: (_, record) => actionLabels[record.action] ?? record.action,
  },
  {
    title: '用户',
    dataIndex: 'username',
    width: 100,
  },
  {
    title: '目标',
    dataIndex: 'targetName',
    width: 160,
    render: (_, record) => record.targetName || record.targetId || '-',
  },
  {
    title: '详情',
    dataIndex: 'detail',
    render: (_, record) => record.detail || '-',
  },
  {
    title: 'IP',
    dataIndex: 'clientIp',
    width: 130,
    render: (_, record) => record.clientIp || '-',
  },
]

export function AuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [category, setCategory] = useState('')
  const [page, setPage] = useState(1)

  const fetchData = useCallback(async (cat: string, currentPage: number) => {
    setLoading(true)
    try {
      const result = await listAuditLogs({
        category: cat || undefined,
        limit: PAGE_SIZE,
        offset: (currentPage - 1) * PAGE_SIZE,
      })
      setLogs(result.items ?? [])
      setTotal(result.total ?? 0)
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载审计日志失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchData(category, page)
  }, [category, page, fetchData])

  function handleCategoryChange(value: string) {
    setCategory(value)
    setPage(1)
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 0 }}
        title="审计日志"
        subTitle="记录系统中所有关键操作，保障数据操作链可溯源"
      />
      {error ? <Typography.Text type="error">{error}</Typography.Text> : null}
      <Space>
        <Select
          style={{ width: 160 }}
          value={category}
          options={categoryOptions}
          onChange={handleCategoryChange}
          placeholder="筛选分类"
        />
      </Space>
      <Table
        columns={columns}
        data={logs}
        rowKey="id"
        loading={loading}
        pagination={{
          total,
          current: page,
          pageSize: PAGE_SIZE,
          onChange: setPage,
          showTotal: true,
        }}
      />
    </Space>
  )
}
