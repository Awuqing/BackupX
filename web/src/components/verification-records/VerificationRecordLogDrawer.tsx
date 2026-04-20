import { Alert, Descriptions, Drawer, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { getVerificationRecord, streamVerificationRecordLogs } from '../../services/verification-records'
import type { BackupLogEvent } from '../../types/backup-records'
import type { VerificationRecordDetail, VerificationRecordStatus } from '../../types/verification-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime, formatDuration } from '../../utils/format'

interface Props {
  visible: boolean
  verifyId?: number
  onCancel: () => void
}

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

function buildLogText(record: VerificationRecordDetail | null, events: BackupLogEvent[]) {
  if (events.length > 0) {
    return events.map((item) => `[${formatDateTime(item.timestamp)}] ${item.message}`).join('\n')
  }
  return record?.logContent ?? ''
}

export function VerificationRecordLogDrawer({ visible, verifyId, onCancel }: Props) {
  const [record, setRecord] = useState<VerificationRecordDetail | null>(null)
  const [events, setEvents] = useState<BackupLogEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [streamError, setStreamError] = useState('')

  useEffect(() => {
    if (!visible || !verifyId) return
    const current = verifyId
    let active = true
    let unsubscribe: (() => void) | null = null

    async function load() {
      setLoading(true)
      try {
        const detail = await getVerificationRecord(current)
        if (!active) return
        setRecord(detail)
        setEvents(detail.logEvents ?? [])
        setError('')
        setStreamError('')
        if (detail.status === 'running') {
          unsubscribe = streamVerificationRecordLogs(current, {
            onEvent: (event) => {
              if (!active) return
              setEvents((existing) => (existing.some((i) => i.sequence === event.sequence) ? existing : [...existing, event]))
              if (event.completed) {
                setRecord((existing) => (existing ? { ...existing, status: event.status as VerificationRecordStatus } : existing))
              }
            },
            onDone: () => {
              if (!active) return
              void (async () => {
                try {
                  const latest = await getVerificationRecord(current)
                  if (active) {
                    setRecord(latest)
                    setEvents(latest.logEvents ?? [])
                  }
                } catch (e) {
                  if (active) setStreamError(resolveErrorMessage(e, '刷新验证详情失败'))
                }
              })()
            },
            onError: (message) => {
              if (active) setStreamError(message)
            },
          })
        }
      } catch (e) {
        if (active) setError(resolveErrorMessage(e, '加载验证记录失败'))
      } finally {
        if (active) setLoading(false)
      }
    }

    void load()
    return () => {
      active = false
      unsubscribe?.()
    }
  }, [verifyId, visible])

  const logText = useMemo(() => buildLogText(record, events), [events, record])

  return (
    <Drawer width={720} title="验证记录详情" visible={visible} onCancel={onCancel} footer={null}>
      {loading ? (
        <Spin />
      ) : error ? (
        <Alert type="error" content={error} />
      ) : record ? (
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          {streamError ? <Alert type="warning" content={streamError} /> : null}
          <div>
            <Typography.Title heading={6} style={{ marginTop: 0, marginBottom: 4 }}>
              {record.taskName}
            </Typography.Title>
            <Space>
              <Tag color={statusColor(record.status)} bordered>{statusLabel(record.status)}</Tag>
              <Tag bordered>{record.mode === 'deep' ? '深度模式' : '快速模式'}</Tag>
              {record.triggeredBy && <Tag bordered>触发: {record.triggeredBy}</Tag>}
            </Space>
          </div>
          <Descriptions
            column={1}
            data={[
              { label: '源备份', value: `#${record.backupRecordId}${record.backupFileName ? ` (${record.backupFileName})` : ''}` },
              { label: '验证摘要', value: record.summary || '-' },
              { label: '开始时间', value: formatDateTime(record.startedAt) },
              { label: '完成时间', value: formatDateTime(record.completedAt) },
              { label: '耗时', value: formatDuration(record.durationSeconds) },
              { label: '错误信息', value: record.errorMessage || '-' },
            ]}
          />
          <div>
            <Typography.Title heading={6}>执行日志</Typography.Title>
            <div className="log-viewer">{logText || '暂无日志输出'}</div>
          </div>
        </Space>
      ) : null}
    </Drawer>
  )
}
