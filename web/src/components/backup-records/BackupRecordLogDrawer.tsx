import { Alert, Button, Descriptions, Drawer, Message, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { deleteBackupRecord, downloadBackupRecord, getBackupRecord, streamBackupRecordLogs } from '../../services/backup-records'
import { getBackupTask } from '../../services/backup-tasks'
import { startRestoreFromBackup } from '../../services/restore-records'
import { startVerifyByRecord } from '../../services/verification-records'
import { useAuthStore } from '../../stores/auth'
import { canWrite } from '../../utils/permissions'
import type { BackupLogEvent, BackupRecordDetail, BackupRecordStatus, StorageUploadResultItem } from '../../types/backup-records'
import type { BackupTaskDetail } from '../../types/backup-tasks'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatDuration } from '../../utils/format'
import { RestoreConfirmModal } from '../restore-records/RestoreConfirmModal'

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
  const navigate = useNavigate()
  const currentUser = useAuthStore((state) => state.user)
  const writable = canWrite(currentUser)
  const [record, setRecord] = useState<BackupRecordDetail | null>(null)
  const [events, setEvents] = useState<BackupLogEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [acting, setActing] = useState(false)
  const [error, setError] = useState('')
  const [streamError, setStreamError] = useState('')
  const [restoreModalVisible, setRestoreModalVisible] = useState(false)
  const [restoreTask, setRestoreTask] = useState<BackupTaskDetail | null>(null)
  const [restoreLoading, setRestoreLoading] = useState(false)
  const [restorePreparing, setRestorePreparing] = useState(false)
  const [verifyLoading, setVerifyLoading] = useState(false)

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

  // handleOpenRestore 准备恢复所需的任务上下文并打开确认弹窗。
  // 只有在用户明确二次确认后，才会真正触发异步恢复流程。
  async function handleOpenRestore() {
    if (!record) {
      return
    }
    setRestorePreparing(true)
    try {
      const task = await getBackupTask(record.taskId)
      setRestoreTask(task)
      setRestoreModalVisible(true)
    } catch (prepareError) {
      Message.error(resolveErrorMessage(prepareError, '加载任务信息失败'))
    } finally {
      setRestorePreparing(false)
    }
  }

  // handleVerify 基于当前备份记录启动一次快速验证，验证结果在"验证演练"页面查看。
  async function handleVerify() {
    if (!recordId) return
    setVerifyLoading(true)
    try {
      const verify = await startVerifyByRecord(recordId, 'quick')
      Message.success('验证已启动，正在打开结果')
      navigate(`/verify/records?verifyId=${verify.id}`)
      onCancel()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '启动验证失败'))
    } finally {
      setVerifyLoading(false)
    }
  }

  async function handleConfirmRestore() {
    if (!recordId) {
      return
    }
    setRestoreLoading(true)
    try {
      const restore = await startRestoreFromBackup(recordId)
      Message.success('恢复已启动，正在打开日志')
      setRestoreModalVisible(false)
      setRestoreTask(null)
      await onChanged?.()
      navigate(`/restore/records?restoreId=${restore.id}`)
      onCancel()
    } catch (restoreError) {
      Message.error(resolveErrorMessage(restoreError, '启动恢复失败'))
    } finally {
      setRestoreLoading(false)
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
            {writable && (
              <Button
                type="primary"
                loading={restorePreparing}
                disabled={record.status !== 'success'}
                onClick={() => void handleOpenRestore()}
              >
                恢复
              </Button>
            )}
            {writable && (
              <Button
                loading={verifyLoading}
                disabled={record.status !== 'success'}
                onClick={() => void handleVerify()}
              >
                验证
              </Button>
            )}
            {writable && (
              <Button loading={acting} status="danger" onClick={handleDelete}>
                删除
              </Button>
            )}
          </Space>
          {record.storageUploadResults && record.storageUploadResults.length > 1 && (
            <div>
              <Typography.Title heading={6}>存储目标上传结果</Typography.Title>
              <Descriptions
                column={1}
                data={record.storageUploadResults.map((r: StorageUploadResultItem) => ({
                  label: r.storageTargetName,
                  value: r.status === 'success' ? '上传成功' : `上传失败: ${r.error || '未知错误'}`,
                }))}
              />
            </div>
          )}

          <div>
            <Typography.Title heading={6}>执行日志</Typography.Title>
            <div className="log-viewer">{logText || '暂无日志输出'}</div>
          </div>
        </Space>
      ) : null}
      <RestoreConfirmModal
        visible={restoreModalVisible}
        loading={restoreLoading}
        backupRecord={record}
        task={restoreTask}
        onCancel={() => {
          if (restoreLoading) return
          setRestoreModalVisible(false)
          setRestoreTask(null)
        }}
        onConfirm={() => void handleConfirmRestore()}
      />
    </Drawer>
  )
}
