import { Card, Empty, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { listReplicationRecords, type ReplicationRecordSummary, type ReplicationStatus } from '../../services/replication-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatDuration } from '../../utils/format'

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

function statusColor(s: ReplicationStatus) {
  switch (s) {
    case 'success': return 'green'
    case 'failed': return 'red'
    default: return 'arcoblue'
  }
}

function statusLabel(s: ReplicationStatus) {
  switch (s) {
    case 'success': return '成功'
    case 'failed': return '失败'
    case 'running': return '执行中'
    default: return s
  }
}

// ReplicationRecordsPage 展示备份复制（3-2-1 规则）执行历史。
export function ReplicationRecordsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [records, setRecords] = useState<ReplicationRecordSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const status = (searchParams.get('status') ?? '') as ReplicationStatus | ''

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setRecords(await listReplicationRecords({ status }))
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载复制记录失败'))
    } finally {
      setLoading(false)
    }
  }, [status])

  useEffect(() => { void load() }, [load])

  function setStatus(v?: string) {
    const next = new URLSearchParams(searchParams)
    if (!v) next.delete('status')
    else next.set('status', v)
    setSearchParams(next, { replace: true })
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>备份复制</Typography.Title>
        <Typography.Paragraph type="secondary">
          3-2-1 规则核心：每份备份至少存在于 2 个独立存储、1 份异地。启用后系统会在每次备份成功后自动镜像到副本目标。
        </Typography.Paragraph>
      </div>

      <Card>
        <Space wrap>
          <div>
            <Typography.Text>状态筛选</Typography.Text>
            <Select style={{ width: 180 }} value={status} options={statusOptions} onChange={(v) => setStatus(v ? String(v) : undefined)} />
          </div>
        </Space>
      </Card>

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}

      <Card>
        {records.length === 0 && !loading ? (
          <Empty description="暂无复制记录" />
        ) : (
          <Table
            rowKey="id"
            loading={loading}
            data={records}
            stripe
            pagination={{ pageSize: 10 }}
            columns={[
              { title: '任务/状态', render: (_: unknown, r: ReplicationRecordSummary) => (
                <Space direction="vertical" size={2}>
                  <Typography.Text bold>任务 #{r.taskId}</Typography.Text>
                  <Tag color={statusColor(r.status)} bordered>{statusLabel(r.status)}</Tag>
                </Space>
              )},
              { title: '源 → 目标', render: (_: unknown, r: ReplicationRecordSummary) => (
                <Space direction="vertical" size={2}>
                  <Typography.Text>{r.sourceTargetName || `#${r.sourceTargetId}`}</Typography.Text>
                  <Typography.Text type="secondary">↓ {r.destTargetName || `#${r.destTargetId}`}</Typography.Text>
                </Space>
              )},
              { title: '大小', dataIndex: 'fileSize', render: (v: number) => formatBytes(v) },
              { title: '耗时', dataIndex: 'durationSeconds', render: (v: number) => formatDuration(v) },
              { title: '触发', dataIndex: 'triggeredBy', render: (v: string) => v || '-' },
              { title: '开始时间', dataIndex: 'startedAt', render: (v: string) => formatDateTime(v) },
              { title: '错误', dataIndex: 'errorMessage', render: (v: string) => v || '-' },
            ]}
          />
        )}
      </Card>
    </Space>
  )
}
