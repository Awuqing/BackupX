import { Button, Card, Empty, Message, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { BackupRecordLogDrawer } from '../../components/backup-records/BackupRecordLogDrawer'
import { listBackupRecords } from '../../services/backup-records'
import { listBackupTasks } from '../../services/backup-tasks'
import type { BackupRecordStatus, BackupRecordSummary } from '../../types/backup-records'
import type { BackupTaskSummary } from '../../types/backup-tasks'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatDuration } from '../../utils/format'

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

function getRecordStatusColor(status: BackupRecordStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'arcoblue'
  }
}

export function BackupRecordsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [records, setRecords] = useState<BackupRecordSummary[]>([])
  const [tasks, setTasks] = useState<BackupTaskSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const selectedTaskId = Number(searchParams.get('taskId') ?? 0) || undefined
  const selectedRecordId = Number(searchParams.get('recordId') ?? 0) || undefined
  const selectedStatus = (searchParams.get('status') ?? '') as BackupRecordStatus | ''

  const taskOptions = useMemo(
    () => [{ label: '全部任务', value: 0 }, ...tasks.map((item) => ({ label: item.name, value: item.id }))],
    [tasks],
  )

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [recordList, taskList] = await Promise.all([
        listBackupRecords({ taskId: selectedTaskId, status: selectedStatus }),
        listBackupTasks(),
      ])
      setRecords(recordList)
      setTasks(taskList)
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载备份记录失败'))
    } finally {
      setLoading(false)
    }
  }, [selectedStatus, selectedTaskId])

  useEffect(() => {
    void loadData()
  }, [loadData])

  function updateSearchParam(key: 'taskId' | 'status' | 'recordId', value?: string) {
    const nextParams = new URLSearchParams(searchParams)
    if (!value || value === '0') {
      nextParams.delete(key)
    } else {
      nextParams.set(key, value)
    }
    setSearchParams(nextParams, { replace: true })
  }

  const columns = [
    {
      title: '任务 / 状态',
      dataIndex: 'taskName',
      render: (_: unknown, record: BackupRecordSummary) => {
        const statusLabel = record.status === 'success' ? '成功' : record.status === 'failed' ? '失败' : record.status === 'running' ? '执行中' : record.status
        return (
          <Space direction="vertical" size={2}>
            <Typography.Text bold>{record.taskName}</Typography.Text>
            <Space>
              {statusLabel ? <Tag color={getRecordStatusColor(record.status)} bordered>{statusLabel}</Tag> : <span style={{ color: 'var(--color-text-3)' }}>-</span>}
              {record.storageTargetName ? <Tag color="arcoblue" bordered>{record.storageTargetName}</Tag> : <span style={{ color: 'var(--color-text-3)' }}>-</span>}
            </Space>
          </Space>
        )
      },
    },
    {
      title: '文件',
      dataIndex: 'fileName',
      render: (_: unknown, record: BackupRecordSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{record.fileName || '-'}</Typography.Text>
          <Typography.Text type="secondary">{formatBytes(record.fileSize)}</Typography.Text>
          {record.checksum && (
            <Typography.Text type="secondary" copyable style={{ fontSize: 11 }}>
              SHA-256: {record.checksum.substring(0, 16)}...
            </Typography.Text>
          )}
        </Space>
      ),
    },
    {
      title: '开始 / 完成',
      dataIndex: 'startedAt',
      render: (_: unknown, record: BackupRecordSummary) => (
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
      title: '错误信息',
      dataIndex: 'errorMessage',
      render: (value: string) => value || '-',
    },
    {
      title: '操作',
      dataIndex: 'actions',
      width: 120,
      render: (_: unknown, record: BackupRecordSummary) => (
        <Button size="small" type="text" onClick={() => updateSearchParam('recordId', String(record.id))}>
          查看日志
        </Button>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>备份记录</Typography.Title>
        <Typography.Paragraph type="secondary">
          查看任务执行结果、筛选历史记录，并在详情中跟踪实时日志、下载或恢复产物。
        </Typography.Paragraph>
      </div>

      <Card>
        <Space wrap>
          <div>
            <Typography.Text>任务筛选</Typography.Text>
            <Select style={{ width: 240 }} value={selectedTaskId ?? 0} options={taskOptions} onChange={(value) => updateSearchParam('taskId', Number(value) > 0 ? String(value) : undefined)} />
          </div>
          <div>
            <Typography.Text>状态筛选</Typography.Text>
            <Select style={{ width: 180 }} value={selectedStatus} options={statusOptions} onChange={(value) => updateSearchParam('status', value ? String(value) : undefined)} />
          </div>
          <Button type="outline" onClick={() => {
            const next = new URLSearchParams(searchParams)
            next.delete('taskId')
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
          <Empty description="暂无备份记录" />
        ) : (
          <Table rowKey="id" loading={loading} columns={columns} data={records} pagination={{ pageSize: 10 }} stripe noDataElement={<Empty description="暂无符合条件的备份记录" />} />
        )}
      </Card>

      <BackupRecordLogDrawer
        visible={Boolean(selectedRecordId)}
        recordId={selectedRecordId}
        onCancel={() => updateSearchParam('recordId', undefined)}
        onChanged={async () => {
          await loadData()
          if (selectedRecordId) {
            Message.success('备份记录已更新')
          }
        }}
      />
    </Space>
  )
}
