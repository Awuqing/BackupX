import { Alert, Button, Descriptions, Drawer, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { deleteBackupRecord, downloadBackupRecord, getBackupRecord, restoreBackupRecord, streamBackupRecordLogs } from '../../services/backup-records'
import type { BackupLogEvent, BackupRecordDetail, BackupRecordStatus } from '../../types/backup-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatDuration } from '../../utils/format'

interface BackupRecordLogDrawerProps {
  visible: boolean
  recordId?: number
  onCancel: () => void
  onChanged?: () => Promise<void> | void
}

function getStatusColor(status: BackupRecordStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'arcoblue'
  }
}

function buildLogText(record: BackupRecordDetail | null, events: BackupLogEvent[]) {
  if (events.length > 0) {
    return events.map((item) => `[${formatDateTime(item.timestamp)}] ${item.message}`).join('\n')
  }
  return record?.logContent ?? ''
}

export function BackupRecordLogDrawer({ visible, recordId, onCancel, onChanged }: BackupRecordLogDrawerProps) {
  const [record, setRecord] = useState<BackupRecordDetail | null>(null)
  const [events, setEvents] = useState<BackupLogEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [acting, setActing] = useState(false)
  const [error, setError] = useState('')
  const [streamError, setStreamError] = useState('')

  useEffect(() => {
    if (!visible || !recordId) {
      return
    }

    const currentRecordId = recordId
    let active = true
    let unsubscribe: (() => void) | null = null

    async function loadRecordDetail() {
      setLoading(true)
      try {
        const detail = await getBackupRecord(currentRecordId)
        if (!active) {
          return
        }
        setRecord(detail)
        setEvents(detail.logEvents ?? [])
        setError('')
        setStreamError('')

        if (detail.status === 'running') {
          unsubscribe = streamBackupRecordLogs(currentRecordId, {
            onEvent: (event) => {
              if (!active) {
                return
              }
              setEvents((current) => {
                if (current.some((item) => item.sequence === event.sequence)) {
                  return current
                }
                return [...current, event]
              })
              if (event.completed) {
                setRecord((current) => (current ? { ...current, status: event.status as BackupRecordStatus } : current))
              }
            },
            onDone: () => {
              if (!active) {
                return
              }
              void (async () => {
                try {
                  const latest = await getBackupRecord(currentRecordId)
                  if (active) {
                    setRecord(latest)
                    setEvents(latest.logEvents ?? [])
                  }
                } catch (streamLoadError) {
                  if (active) {
                    setStreamError(resolveErrorMessage(streamLoadError, '刷新日志详情失败'))
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
          setError(resolveErrorMessage(loadError, '加载备份记录失败'))
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadRecordDetail()

    return () => {
      active = false
      unsubscribe?.()
    }
  }, [recordId, visible])

  const logText = useMemo(() => buildLogText(record, events), [events, record])

  async function handleDownload() {
    if (!recordId) {
      return
    }
    setActing(true)
    try {
      const result = await downloadBackupRecord(recordId)
      const url = window.URL.createObjectURL(result.blob)
      const link = document.createElement('a')
      link.href = url
      link.download = result.fileName
      link.click()
      window.URL.revokeObjectURL(url)
    } catch (downloadError) {
      setStreamError(resolveErrorMessage(downloadError, '下载备份文件失败'))
    } finally {
      setActing(false)
    }
  }

  async function handleRestore() {
    if (!recordId) {
      return
    }
    setActing(true)
    try {
      await restoreBackupRecord(recordId)
      setStreamError('恢复命令已提交')
      await onChanged?.()
    } catch (restoreError) {
      setStreamError(resolveErrorMessage(restoreError, '恢复备份失败'))
    } finally {
      setActing(false)
    }
  }

  async function handleDelete() {
    if (!recordId) {
      return
    }
    if (!window.confirm('确定删除该备份记录及远端对象吗？')) {
      return
    }
    setActing(true)
    try {
      await deleteBackupRecord(recordId)
      await onChanged?.()
      onCancel()
    } catch (deleteError) {
      setStreamError(resolveErrorMessage(deleteError, '删除备份记录失败'))
    } finally {
      setActing(false)
    }
  }

  return (
    <Drawer width={720} title="备份记录详情" visible={visible} onCancel={onCancel}>
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
              {record.status && (
                <Tag color={getStatusColor(record.status)} bordered>
                  {record.status === 'success' ? '成功' : record.status === 'failed' ? '失败' : record.status === 'running' ? '执行中' : record.status}
                </Tag>
              )}
              {record.storageTargetName && <Tag color="arcoblue" bordered>{record.storageTargetName}</Tag>}
            </Space>
          </div>
          <Descriptions
            column={1}
            data={[
              { label: '文件名', value: record.fileName || '-' },
              { label: '文件大小', value: formatBytes(record.fileSize) },
              { label: '存储路径', value: record.storagePath || '-' },
              { label: '开始时间', value: formatDateTime(record.startedAt) },
              { label: '完成时间', value: formatDateTime(record.completedAt) },
              { label: '耗时', value: formatDuration(record.durationSeconds) },
              { label: '错误信息', value: record.errorMessage || '-' },
            ]}
          />
          <Space>
            <Button loading={acting} onClick={handleDownload}>
              下载
            </Button>
            <Button loading={acting} onClick={handleRestore}>
              恢复
            </Button>
            <Button loading={acting} status="danger" onClick={handleDelete}>
              删除
            </Button>
          </Space>
          <div>
            <Typography.Title heading={6}>执行日志</Typography.Title>
            <div className="log-viewer">{logText || '暂无日志输出'}</div>
          </div>
        </Space>
      ) : null}
    </Drawer>
  )
}
