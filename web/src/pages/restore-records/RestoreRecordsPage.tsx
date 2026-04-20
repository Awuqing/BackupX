import { Button, Card, Empty, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { RestoreRecordLogDrawer } from '../../components/restore-records/RestoreRecordLogDrawer'
import { listRestoreRecords } from '../../services/restore-records'
import type { RestoreRecordStatus, RestoreRecordSummary } from '../../types/restore-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime, formatDuration } from '../../utils/format'

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

function statusColor(status: RestoreRecordStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'arcoblue'
  }
}

function statusLabel(status: RestoreRecordStatus) {
  switch (status) {
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'running':
      return '执行中'
    default:
      return status
  }
}

export function RestoreRecordsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [records, setRecords] = useState<RestoreRecordSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const selectedRestoreId = Number(searchParams.get('restoreId') ?? 0) || undefined
  const selectedStatus = (searchParams.get('status') ?? '') as RestoreRecordStatus | ''

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const items = await listRestoreRecords({ status: selectedStatus })
      setRecords(items)
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载恢复记录失败'))
    } finally {
      setLoading(false)
    }
  }, [selectedStatus])

  useEffect(() => {
    void loadData()
  }, [loadData])

  function updateSearchParam(key: 'status' | 'restoreId', value?: string) {
    const next = new URLSearchParams(searchParams)
    if (!value) {
      next.delete(key)
    } else {
      next.set(key, value)
    }
    setSearchParams(next, { replace: true })
  }

  const columns = [
    {
      title: '任务 / 状态',
      dataIndex: 'taskName',
      render: (_: unknown, record: RestoreRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text bold>{record.taskName}</Typography.Text>
          <Space>
            <Tag color={statusColor(record.status)} bordered>{statusLabel(record.status)}</Tag>
            {record.nodeName ? (
              <Tag color="arcoblue" bordered>{record.nodeName}</Tag>
            ) : record.nodeId === 0 ? (
              <Tag color="arcoblue" bordered>本机 Master</Tag>
            ) : null}
          </Space>
        </Space>
      ),
    },
    {
      title: '源备份',
      render: (_: unknown, record: RestoreRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{record.backupFileName || `#${record.backupRecordId}`}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>备份记录 ID: {record.backupRecordId}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '开始 / 完成',
      dataIndex: 'startedAt',
      render: (_: unknown, record: RestoreRecordSummary) => (
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
      title: '触发人',
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
      render: (_: unknown, record: RestoreRecordSummary) => (
        <Button size="small" type="text" onClick={() => updateSearchParam('restoreId', String(record.id))}>
          查看日志
        </Button>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>恢复记录</Typography.Title>
        <Typography.Paragraph type="secondary">
          查看备份恢复的执行结果与实时日志。恢复会在任务绑定的节点上执行（本机 Master 或远程 Agent）。
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
          <Empty description="暂无恢复记录" />
        ) : (
          <Table rowKey="id" loading={loading} columns={columns} data={records} pagination={{ pageSize: 10 }} stripe noDataElement={<Empty description="暂无符合条件的恢复记录" />} />
        )}
      </Card>

      <RestoreRecordLogDrawer
        visible={Boolean(selectedRestoreId)}
        restoreId={selectedRestoreId}
        onCancel={() => updateSearchParam('restoreId', undefined)}
      />
    </Space>
  )
}
