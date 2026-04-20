import { Alert, Descriptions, Drawer, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { getRestoreRecord, streamRestoreRecordLogs } from '../../services/restore-records'
import type { BackupLogEvent } from '../../types/backup-records'
import type { RestoreRecordDetail, RestoreRecordStatus } from '../../types/restore-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime, formatDuration } from '../../utils/format'

interface RestoreRecordLogDrawerProps {
  visible: boolean
  restoreId?: number
  onCancel: () => void
}

function getStatusColor(status: RestoreRecordStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'arcoblue'
  }
}

function buildLogText(record: RestoreRecordDetail | null, events: BackupLogEvent[]) {
  if (events.length > 0) {
    return events.map((item) => `[${formatDateTime(item.timestamp)}] ${item.message}`).join('\n')
  }
  return record?.logContent ?? ''
}

export function RestoreRecordLogDrawer({ visible, restoreId, onCancel }: RestoreRecordLogDrawerProps) {
  const [record, setRecord] = useState<RestoreRecordDetail | null>(null)
  const [events, setEvents] = useState<BackupLogEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [streamError, setStreamError] = useState('')

  useEffect(() => {
    if (!visible || !restoreId) {
      return
    }

    const currentId = restoreId
    let active = true
    let unsubscribe: (() => void) | null = null

    async function loadDetail() {
      setLoading(true)
      try {
        const detail = await getRestoreRecord(currentId)
        if (!active) {
          return
        }
        setRecord(detail)
        setEvents(detail.logEvents ?? [])
        setError('')
        setStreamError('')

        if (detail.status === 'running') {
          unsubscribe = streamRestoreRecordLogs(currentId, {
            onEvent: (event) => {
              if (!active) return
              setEvents((current) => {
                if (current.some((item) => item.sequence === event.sequence)) {
                  return current
                }
                return [...current, event]
              })
              if (event.completed) {
                setRecord((current) => (current ? { ...current, status: event.status as RestoreRecordStatus } : current))
              }
            },
            onDone: () => {
              if (!active) return
              void (async () => {
                try {
                  const latest = await getRestoreRecord(currentId)
                  if (active) {
                    setRecord(latest)
                    setEvents(latest.logEvents ?? [])
                  }
                } catch (refreshError) {
                  if (active) {
                    setStreamError(resolveErrorMessage(refreshError, '刷新恢复详情失败'))
                  }
                }
              })()
            },
            onError: (message) => {
              if (active) {
                setStreamError(message)
              }
            },
          })
        }
      } catch (loadError) {
        if (active) {
          setError(resolveErrorMessage(loadError, '加载恢复记录失败'))
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadDetail()

    return () => {
      active = false
      unsubscribe?.()
    }
  }, [restoreId, visible])

  const logText = useMemo(() => buildLogText(record, events), [events, record])

  return (
    <Drawer width={720} title="恢复记录详情" visible={visible} onCancel={onCancel} footer={null}>
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
              <Tag color={getStatusColor(record.status)} bordered>
                {record.status === 'success' ? '成功' : record.status === 'failed' ? '失败' : '执行中'}
              </Tag>
              {record.nodeName ? (
                <Tag color="arcoblue" bordered>节点: {record.nodeName}</Tag>
              ) : record.nodeId === 0 ? (
                <Tag color="arcoblue" bordered>节点: 本机 Master</Tag>
              ) : null}
              {record.triggeredBy && <Tag bordered>触发人: {record.triggeredBy}</Tag>}
            </Space>
          </div>
          <Descriptions
            column={1}
            data={[
              { label: '源备份记录', value: `#${record.backupRecordId}${record.backupFileName ? ` (${record.backupFileName})` : ''}` },
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
