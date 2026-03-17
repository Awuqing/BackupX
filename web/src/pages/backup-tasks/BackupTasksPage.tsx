import { Button, Card, Empty, Message, PageHeader, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BackupTaskDetailDrawer } from '../../components/backup-tasks/BackupTaskDetailDrawer'
import { BackupTaskFormDrawer } from '../../components/backup-tasks/BackupTaskFormDrawer'
import { getBackupTaskStatusColor, getBackupTaskStatusLabel, getBackupTaskTypeLabel } from '../../components/backup-tasks/field-config'
import { createBackupTask, deleteBackupTask, getBackupTask, listBackupTasks, runBackupTask, toggleBackupTask, updateBackupTask } from '../../services/backup-tasks'
import { listStorageTargets } from '../../services/storage-targets'
import type { BackupTaskDetail, BackupTaskPayload, BackupTaskSummary } from '../../types/backup-tasks'
import type { StorageTargetSummary } from '../../types/storage-targets'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime } from '../../utils/format'

export function BackupTasksPage() {
  const navigate = useNavigate()
  const [tasks, setTasks] = useState<BackupTaskSummary[]>([])
  const [storageTargets, setStorageTargets] = useState<StorageTargetSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [drawerVisible, setDrawerVisible] = useState(false)
  const [detailVisible, setDetailVisible] = useState(false)
  const [editingTask, setEditingTask] = useState<BackupTaskDetail | null>(null)
  const [detailTask, setDetailTask] = useState<BackupTaskDetail | null>(null)
  const [error, setError] = useState('')

  const enabledStorageTargets = useMemo(() => storageTargets.filter((item) => item.enabled), [storageTargets])

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [taskList, targetList] = await Promise.all([listBackupTasks(), listStorageTargets()])
      setTasks(taskList)
      setStorageTargets(targetList)
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载备份任务失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadData()
  }, [loadData])

  async function openEdit(id: number) {
    setSubmitting(true)
    try {
      const detail = await getBackupTask(id)
      setEditingTask(detail)
      setDrawerVisible(true)
    } catch (loadError) {
      Message.error(resolveErrorMessage(loadError, '加载任务详情失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function openDetail(id: number) {
    setSubmitting(true)
    try {
      const detail = await getBackupTask(id)
      setDetailTask(detail)
      setDetailVisible(true)
    } catch (loadError) {
      Message.error(resolveErrorMessage(loadError, '加载任务详情失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSubmit(value: BackupTaskPayload, taskId?: number) {
    setSubmitting(true)
    try {
      if (taskId) {
        await updateBackupTask(taskId, value)
        Message.success('备份任务已更新')
      } else {
        await createBackupTask(value)
        Message.success('备份任务已创建')
      }
      setDrawerVisible(false)
      setEditingTask(null)
      await loadData()
    } catch (submitError) {
      Message.error(resolveErrorMessage(submitError, '保存备份任务失败'))
      throw submitError
    } finally {
      setSubmitting(false)
    }
  }

  async function handleToggle(task: BackupTaskSummary) {
    try {
      await toggleBackupTask(task.id, { enabled: !task.enabled })
      Message.success(task.enabled ? '任务已停用' : '任务已启用')
      await loadData()
    } catch (toggleError) {
      Message.error(resolveErrorMessage(toggleError, '切换任务状态失败'))
    }
  }

  async function handleRun(task: BackupTaskSummary) {
    try {
      const record = await runBackupTask(task.id)
      Message.success('已触发备份任务，正在打开执行日志')
      navigate(`/backup/records?taskId=${task.id}&recordId=${record.id}`)
    } catch (runError) {
      Message.error(resolveErrorMessage(runError, '触发备份任务失败'))
    }
  }

  async function handleDelete(task: BackupTaskSummary) {
    if (!window.confirm(`确定删除任务“${task.name}”吗？`)) {
      return
    }
    try {
      await deleteBackupTask(task.id)
      Message.success('备份任务已删除')
      await loadData()
    } catch (deleteError) {
      Message.error(resolveErrorMessage(deleteError, '删除备份任务失败'))
    }
  }

  const columns = [
    {
      title: '任务名称',
      dataIndex: 'name',
      render: (_: unknown, record: BackupTaskSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text bold>{record.name}</Typography.Text>
          <Space>
            {getBackupTaskTypeLabel(record.type) && <Tag color="arcoblue" bordered>{getBackupTaskTypeLabel(record.type)}</Tag>}
            {record.enabled !== undefined && (
              <Tag color={record.enabled ? 'green' : 'gray'} bordered>{record.enabled ? '已启用' : '已停用'}</Tag>
            )}
          </Space>
        </Space>
      ),
    },
    {
      title: '调度',
      dataIndex: 'cronExpr',
      render: (value: string) => value || '仅手动执行',
    },
    {
      title: '存储目标',
      dataIndex: 'storageTargetName',
      render: (value: string) => value || '-',
    },
    {
      title: '策略',
      dataIndex: 'retentionDays',
      render: (_: unknown, record: BackupTaskSummary) => `${record.retentionDays} 天 / ${record.maxBackups} 份`,
    },
    {
      title: '最近状态',
      render: (value: BackupTaskSummary['lastStatus']) => {
        const label = getBackupTaskStatusLabel(value)
        return label ? <Tag color={getBackupTaskStatusColor(value)} bordered>{label}</Tag> : <span style={{ color: 'var(--color-text-3)' }}>-</span>
      },
    },
    {
      title: '最近执行',
      dataIndex: 'lastRunAt',
      render: (value?: string) => formatDateTime(value),
    },
    {
      title: '操作',
      dataIndex: 'actions',
      width: 280,
      render: (_: unknown, record: BackupTaskSummary) => (
        <Space wrap size="mini">
          <Button size="small" type="text" onClick={() => void openDetail(record.id)}>
            详情
          </Button>
          <Button size="small" type="text" onClick={() => void openEdit(record.id)} loading={submitting && editingTask?.id === record.id}>
            编辑
          </Button>
          <Button size="small" type="text" status="success" onClick={() => void handleRun(record)}>
            立即执行
          </Button>
          <Button size="small" type="text" onClick={() => void handleToggle(record)}>
            {record.enabled ? '停用' : '启用'}
          </Button>
          <Button size="small" type="text" status="danger" onClick={() => void handleDelete(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 16 }}
        title="备份任务"
        subTitle="管理文件目录、MySQL、SQLite 与 PostgreSQL 的备份计划，并支持立即执行"
        extra={
          <Button
            type="primary"
            disabled={enabledStorageTargets.length === 0}
            onClick={() => {
              setEditingTask(null)
              setDrawerVisible(true)
            }}
          >
            新建任务
          </Button>
        }
      />

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}
      {enabledStorageTargets.length === 0 ? (
        <Card>
          <Empty description="请先启用至少一个存储目标，再创建备份任务。" />
        </Card>
      ) : null}

      <Card>
        <Table rowKey="id" loading={loading} columns={columns} data={tasks} pagination={{ pageSize: 10 }} stripe noDataElement={<Empty description="暂无备份任务，请先点击右上角创建任务" />} />
      </Card>

      <BackupTaskFormDrawer
        visible={drawerVisible}
        loading={submitting}
        initialValue={editingTask}
        storageTargets={enabledStorageTargets}
        onCancel={() => {
          setDrawerVisible(false)
          setEditingTask(null)
        }}
        onSubmit={handleSubmit}
      />

      <BackupTaskDetailDrawer
        visible={detailVisible}
        task={detailTask}
        onCancel={() => {
          setDetailVisible(false)
          setDetailTask(null)
        }}
      />
    </Space>
  )
}
