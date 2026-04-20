import { Button, Card, Empty, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { VerificationRecordLogDrawer } from '../../components/verification-records/VerificationRecordLogDrawer'
import { listVerificationRecords } from '../../services/verification-records'
import type { VerificationRecordStatus, VerificationRecordSummary } from '../../types/verification-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime, formatDuration } from '../../utils/format'

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '验证中', value: 'running' },
  { label: '通过', value: 'success' },
  { label: '未通过', value: 'failed' },
]

function statusColor(status: VerificationRecordStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'arcoblue'
  }
}

function statusLabel(status: VerificationRecordStatus) {
  switch (status) {
    case 'success':
      return '通过'
    case 'failed':
      return '未通过'
    case 'running':
      return '验证中'
    default:
      return status
  }
}

export function VerificationRecordsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [records, setRecords] = useState<VerificationRecordSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const selectedVerifyId = Number(searchParams.get('verifyId') ?? 0) || undefined
  const selectedStatus = (searchParams.get('status') ?? '') as VerificationRecordStatus | ''

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const items = await listVerificationRecords({ status: selectedStatus })
      setRecords(items)
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载验证记录失败'))
    } finally {
      setLoading(false)
    }
  }, [selectedStatus])

  useEffect(() => {
    void loadData()
  }, [loadData])

  function updateSearchParam(key: 'status' | 'verifyId', value?: string) {
    const next = new URLSearchParams(searchParams)
    if (!value) next.delete(key)
    else next.set(key, value)
    setSearchParams(next, { replace: true })
  }

  const columns = [
    {
      title: '任务 / 结果',
      render: (_: unknown, record: VerificationRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text bold>{record.taskName}</Typography.Text>
          <Space>
            <Tag color={statusColor(record.status)} bordered>{statusLabel(record.status)}</Tag>
            <Tag bordered>{record.mode === 'deep' ? '深度' : '快速'}</Tag>
          </Space>
        </Space>
      ),
    },
    {
      title: '摘要 / 源备份',
      render: (_: unknown, record: VerificationRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{record.summary || '-'}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            源备份 #{record.backupRecordId}{record.backupFileName ? ` (${record.backupFileName})` : ''}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '开始 / 完成',
      render: (_: unknown, record: VerificationRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{formatDateTime(record.startedAt)}</Typography.Text>
          <Typography.Text type="secondary">{formatDateTime(record.completedAt)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'durationSeconds',
      render: (value: number) => formatDuration(value),
    },
    {
      title: '触发',
      dataIndex: 'triggeredBy',
      render: (value: string) => value || '-',
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      render: (value: string) => value || '-',
    },
    {
      title: '操作',
      dataIndex: 'actions',
      width: 120,
      render: (_: unknown, record: VerificationRecordSummary) => (
        <Button size="small" type="text" onClick={() => updateSearchParam('verifyId', String(record.id))}>
          查看日志
        </Button>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>验证演练</Typography.Title>
        <Typography.Paragraph type="secondary">
          自动化校验备份的可恢复性（企业合规刚需）。定时从最新成功备份执行完整性/格式校验，不改动任何源数据。
        </Typography.Paragraph>
      </div>

      <Card>
        <Space wrap>
          <div>
            <Typography.Text>状态筛选</Typography.Text>
            <Select style={{ width: 180 }} value={selectedStatus} options={statusOptions} onChange={(value) => updateSearchParam('status', value ? String(value) : undefined)} />
          </div>
          <Button type="outline" onClick={() => {
            const next = new URLSearchParams(searchParams)
            next.delete('status')
            setSearchParams(next, { replace: true })
          }}>
            重置筛选
          </Button>
        </Space>
      </Card>

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}

      <Card>
        {records.length === 0 && !loading ? (
          <Empty description="暂无验证记录" />
        ) : (
          <Table rowKey="id" loading={loading} columns={columns} data={records} pagination={{ pageSize: 10 }} stripe noDataElement={<Empty description="暂无验证记录" />} />
        )}
      </Card>

      <VerificationRecordLogDrawer
        visible={Boolean(selectedVerifyId)}
        verifyId={selectedVerifyId}
        onCancel={() => updateSearchParam('verifyId', undefined)}
      />
    </Space>
  )
}
