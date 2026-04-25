import { Button, DatePicker, Input, Message, PageHeader, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import type { ColumnProps } from '@arco-design/web-react/es/Table'
import { useCallback, useEffect, useState } from 'react'
import { exportAuditLogs, listAuditLogs } from '../../services/audit'
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
  two_factor_required: '需要 MFA',
  two_factor_setup: '生成 TOTP',
  two_factor_enable: '启用 TOTP',
  two_factor_disable: '关闭 TOTP',
  two_factor_recovery_code_used: '使用恢复码',
  two_factor_recovery_codes_regenerate: '重建恢复码',
  webauthn_register: '注册通行密钥',
  webauthn_used: '使用通行密钥',
  webauthn_delete: '删除通行密钥',
  trusted_device_create: '信任设备',
  trusted_device_used: '使用可信设备',
  trusted_device_revoke: '移除可信设备',
  otp_enable: '启用 OTP',
  otp_disable: '关闭 OTP',
  otp_send: '发送 OTP',
  otp_used: '使用 OTP',
  reset_two_factor: '重置 MFA',
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
  const [username, setUsername] = useState('')
  const [keyword, setKeyword] = useState('')
  const [dateRange, setDateRange] = useState<string[] | null>(null)
  const [page, setPage] = useState(1)
  const [exporting, setExporting] = useState(false)

  const fetchData = useCallback(async (currentPage: number) => {
    setLoading(true)
    try {
      const result = await listAuditLogs({
        category: category || undefined,
        username: username.trim() || undefined,
        keyword: keyword.trim() || undefined,
        dateFrom: dateRange?.[0] ? new Date(dateRange[0]).toISOString() : undefined,
        dateTo: dateRange?.[1] ? new Date(dateRange[1]).toISOString() : undefined,
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
  }, [category, username, keyword, dateRange])

  useEffect(() => {
    void fetchData(page)
  }, [page, fetchData])

  async function handleExport() {
    setExporting(true)
    try {
      await exportAuditLogs({
        category: category || undefined,
        username: username.trim() || undefined,
        keyword: keyword.trim() || undefined,
        dateFrom: dateRange?.[0] ? new Date(dateRange[0]).toISOString() : undefined,
        dateTo: dateRange?.[1] ? new Date(dateRange[1]).toISOString() : undefined,
      })
      Message.success('CSV 已开始下载')
    } catch (e) {
      Message.error(resolveErrorMessage(e, '导出失败'))
    } finally {
      setExporting(false)
    }
  }

  function handleReset() {
    setCategory('')
    setUsername('')
    setKeyword('')
    setDateRange(null)
    setPage(1)
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 0 }}
        title="审计日志"
        subTitle="记录系统中所有关键操作，保障数据操作链可溯源。支持高级筛选与 CSV 导出（最多 10000 行）。"
      />
      {error ? <Typography.Text type="error">{error}</Typography.Text> : null}
      <Space wrap>
        <Select
          style={{ width: 160 }}
          value={category}
          options={categoryOptions}
          onChange={(v) => { setCategory(v); setPage(1) }}
          placeholder="分类"
        />
        <Input
          style={{ width: 160 }}
          value={username}
          placeholder="用户名"
          onChange={setUsername}
          onPressEnter={() => { setPage(1); void fetchData(1) }}
        />
        <Input
          style={{ width: 240 }}
          value={keyword}
          placeholder="关键词（详情/目标名）"
          onChange={setKeyword}
          onPressEnter={() => { setPage(1); void fetchData(1) }}
        />
        <DatePicker.RangePicker
          showTime
          value={dateRange ?? undefined}
          onChange={(v) => { setDateRange(v as string[] | null); setPage(1) }}
        />
        <Button type="primary" onClick={() => { setPage(1); void fetchData(1) }}>查询</Button>
        <Button onClick={handleReset}>重置</Button>
        <Button type="outline" loading={exporting} onClick={() => void handleExport()}>导出 CSV</Button>
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
