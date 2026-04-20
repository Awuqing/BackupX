import { Button, Card, Empty, Message, Modal, PageHeader, Select, Space, Table, Tag, Typography, Upload } from '@arco-design/web-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { BackupTaskDetailDrawer } from '../../components/backup-tasks/BackupTaskDetailDrawer'
import { BackupTaskFormDrawer } from '../../components/backup-tasks/BackupTaskFormDrawer'
import { TaskDependencyGraph } from '../../components/backup-tasks/TaskDependencyGraph'
import { getBackupTaskStatusColor, getBackupTaskStatusLabel, getBackupTaskTypeLabel } from '../../components/backup-tasks/field-config'
import { batchDeleteTasks, batchRunTasks, batchToggleTasks, createBackupTask, deleteBackupTask, exportBackupTasks, getBackupTask, importBackupTasks, listBackupTasks, runBackupTask, toggleBackupTask, updateBackupTask, type TaskImportResult } from '../../services/backup-tasks'
import { listNodes } from '../../services/nodes'
import { createStorageTarget, listStorageTargets, startGoogleDriveAuth, testStorageTarget } from '../../services/storage-targets'
import type { BackupTaskDetail, BackupTaskPayload, BackupTaskSummary } from '../../types/backup-tasks'
import type { NodeSummary } from '../../types/nodes'
import type { StorageTargetPayload, StorageTargetSummary } from '../../types/storage-targets'
import { useAuthStore } from '../../stores/auth'
import { resolveErrorMessage } from '../../utils/error'
import { canWrite } from '../../utils/permissions'
import { formatDateTime } from '../../utils/format'

export function BackupTasksPage() {
  const navigate = useNavigate()
  const currentUser = useAuthStore((state) => state.user)
  const writable = canWrite(currentUser)
  const [tasks, setTasks] = useState<BackupTaskSummary[]>([])
  const [storageTargets, setStorageTargets] = useState<StorageTargetSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [drawerVisible, setDrawerVisible] = useState(false)
  const [detailVisible, setDetailVisible] = useState(false)
  const [editingTask, setEditingTask] = useState<BackupTaskDetail | null>(null)
  const [detailTask, setDetailTask] = useState<BackupTaskDetail | null>(null)
  const [error, setError] = useState('')
  const [localNodeId, setLocalNodeId] = useState<number | undefined>(undefined)
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [tagFilter, setTagFilter] = useState<string[]>([])
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [batchLoading, setBatchLoading] = useState(false)
  const [importResults, setImportResults] = useState<TaskImportResult[] | null>(null)

  const enabledStorageTargets = useMemo(() => storageTargets.filter((item) => item.enabled), [storageTargets])

  // 从全量任务中提取所有用过的标签，作为筛选器选项
  const availableTags = useMemo(() => {
    const set = new Set<string>()
    for (const task of tasks) {
      if (!task.tags) continue
      for (const tag of task.tags.split(',').map((t) => t.trim()).filter(Boolean)) {
        set.add(tag)
      }
    }
    return Array.from(set).sort()
  }, [tasks])

  // 按标签筛选
  const filteredTasks = useMemo(() => {
    if (tagFilter.length === 0) return tasks
    return tasks.filter((task) => {
      const taskTags = (task.tags ?? '').split(',').map((t) => t.trim()).filter(Boolean)
      return tagFilter.every((filter) => taskTags.includes(filter))
    })
  }, [tasks, tagFilter])

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [taskList, targetList, nodeList] = await Promise.all([listBackupTasks(), listStorageTargets(), listNodes()])
      setTasks(taskList)
      setStorageTargets(targetList)
      setNodes(nodeList)
      const localNode = nodeList.find((n) => n.isLocal)
      if (localNode) {
        setLocalNodeId(localNode.id)
      }
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

  // 导出选中或全部任务为 JSON
  async function handleExport() {
    try {
      await exportBackupTasks(selectedIds.length > 0 ? selectedIds : undefined)
      Message.success(selectedIds.length > 0 ? `已导出 ${selectedIds.length} 个任务` : '已导出全部任务')
    } catch (e) {
      Message.error(resolveErrorMessage(e, '导出失败'))
    }
  }

  // 上传 JSON 并导入任务
  async function handleImport(file: File): Promise<boolean> {
    try {
      const text = await file.text()
      const payload = JSON.parse(text)
      const results = await importBackupTasks(payload)
      setImportResults(results)
      const succ = results.filter((r) => r.success && !r.skipped).length
      const skipped = results.filter((r) => r.skipped).length
      Message.success(`导入完成：创建 ${succ} / 跳过 ${skipped} / 失败 ${results.length - succ - skipped}`)
      await loadData()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '导入失败'))
    }
    return false // 阻止 Arco Upload 自动上传
  }

  // 批量操作辅助
  async function runBatch(
    action: 'run' | 'enable' | 'disable' | 'delete',
  ) {
    if (selectedIds.length === 0) {
      Message.info('请先选择要操作的任务')
      return
    }
    if (action === 'delete' && !window.confirm(`确定删除 ${selectedIds.length} 个任务？操作不可撤销。`)) {
      return
    }
    setBatchLoading(true)
    try {
      let results
      switch (action) {
        case 'run':
          results = await batchRunTasks(selectedIds)
          break
        case 'enable':
          results = await batchToggleTasks(selectedIds, true)
          break
        case 'disable':
          results = await batchToggleTasks(selectedIds, false)
          break
        case 'delete':
          results = await batchDeleteTasks(selectedIds)
          break
      }
      const succ = results.filter((r) => r.success).length
      const fail = results.length - succ
      if (fail === 0) {
        Message.success(`成功处理 ${succ} 个任务`)
      } else {
        Message.warning(`成功 ${succ} / 失败 ${fail}，详情见通知`)
      }
      setSelectedIds([])
      await loadData()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '批量操作失败'))
    } finally {
      setBatchLoading(false)
    }
  }

  async function handleCreateStorageTarget(value: StorageTargetPayload) {
    const result = await createStorageTarget(value)
    Message.success('存储目标已创建')
    return result
  }

  async function handleTestStorageTarget(value: StorageTargetPayload) {
    const result = await testStorageTarget(value)
    Message.success(result.message)
    return result
  }

  async function handleGoogleDriveAuth(value: StorageTargetPayload, targetId?: number) {
    const result = await startGoogleDriveAuth(value, targetId)
    window.open(result.authUrl, '_blank')
  }

  async function reloadStorageTargets() {
    const targetList = await listStorageTargets()
    setStorageTargets(targetList)
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
      dataIndex: 'storageTargetNames',
      render: (_: unknown, record: BackupTaskSummary) => {
        const names = record.storageTargetNames?.length > 0 ? record.storageTargetNames : record.storageTargetName ? [record.storageTargetName] : []
        if (names.length === 0) return '-'
        return (
          <Space size={4} wrap>
            {names.map((name, i) => (
              <Tag key={i} color="arcoblue" bordered>{name}</Tag>
            ))}
          </Space>
        )
      },
    },
    {
      title: '策略',
      dataIndex: 'retentionDays',
      render: (_: unknown, record: BackupTaskSummary) => `${record.retentionDays} 天 / ${record.maxBackups} 份`,
    },
    {
      title: '标签',
      dataIndex: 'tags',
      render: (value: string) => {
        const items = (value ?? '').split(',').map((t) => t.trim()).filter(Boolean)
        if (items.length === 0) return <span style={{ color: 'var(--color-text-3)' }}>-</span>
        return (
          <Space size={4} wrap>
            {items.map((tag) => <Tag key={tag} color="gray" bordered size="small">{tag}</Tag>)}
          </Space>
        )
      },
    },
    {
      title: 'SLA',
      dataIndex: 'slaHoursRpo',
      render: (value: number, record: BackupTaskSummary) => {
        if (value <= 0) return <span style={{ color: 'var(--color-text-3)' }}>未配置</span>
        // 简单着色：仅根据是否启用验证/SLA 显示徽章（实时 SLA 违约见 Dashboard）
        const bits = [<Tag key="rpo" color="arcoblue" bordered size="small">RPO {value}h</Tag>]
        if (record.verifyEnabled) bits.push(<Tag key="verify" color="green" bordered size="small">定时验证</Tag>)
        return <Space size={4} wrap>{bits}</Space>
      },
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
          {writable && (
            <Button size="small" type="text" onClick={() => void openEdit(record.id)} loading={submitting && editingTask?.id === record.id}>
              编辑
            </Button>
          )}
          {writable && (
            <Button size="small" type="text" status="success" onClick={() => void handleRun(record)}>
              立即执行
            </Button>
          )}
          {writable && (
            <Button size="small" type="text" onClick={() => void handleToggle(record)}>
              {record.enabled ? '停用' : '启用'}
            </Button>
          )}
          {writable && (
            <Button size="small" type="text" status="danger" onClick={() => void handleDelete(record)}>
              删除
            </Button>
          )}
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
          <Space>
            <Button size="small" onClick={() => void handleExport()}>
              导出 JSON
            </Button>
            {writable && (
              <Upload
                accept=".json"
                showUploadList={false}
                beforeUpload={(file) => handleImport(file)}
              >
                <Button size="small">导入 JSON</Button>
              </Upload>
            )}
            {writable && (
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
            )}
          </Space>
        }
      />

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}
      {enabledStorageTargets.length === 0 ? (
        <Card>
          <Empty description="请先启用至少一个存储目标，再创建备份任务。" />
        </Card>
      ) : null}

      <TaskDependencyGraph tasks={tasks} />

      {availableTags.length > 0 && (
        <Card size="small">
          <Space wrap>
            <Typography.Text type="secondary" style={{ fontSize: 13 }}>按标签筛选:</Typography.Text>
            <Select
              mode="multiple"
              placeholder="选择标签进行过滤（多标签取交集）"
              style={{ minWidth: 300 }}
              value={tagFilter}
              options={availableTags.map((tag) => ({ label: tag, value: tag }))}
              onChange={(values) => setTagFilter(values as string[])}
              allowClear
            />
            {tagFilter.length > 0 && (
              <Button size="small" type="text" onClick={() => setTagFilter([])}>
                清空筛选
              </Button>
            )}
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              共 {filteredTasks.length} / {tasks.length} 个任务
            </Typography.Text>
          </Space>
        </Card>
      )}

      {writable && selectedIds.length > 0 && (
        <Card size="small" style={{ backgroundColor: 'var(--color-fill-2)' }}>
          <Space wrap>
            <Typography.Text bold>已选 {selectedIds.length} 个任务：</Typography.Text>
            <Button size="small" type="primary" loading={batchLoading} onClick={() => void runBatch('run')}>批量执行</Button>
            <Button size="small" loading={batchLoading} onClick={() => void runBatch('enable')}>批量启用</Button>
            <Button size="small" loading={batchLoading} onClick={() => void runBatch('disable')}>批量停用</Button>
            <Button size="small" status="danger" loading={batchLoading} onClick={() => void runBatch('delete')}>批量删除</Button>
            <Button size="small" type="text" onClick={() => setSelectedIds([])}>取消</Button>
          </Space>
        </Card>
      )}

      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          data={filteredTasks}
          pagination={{ pageSize: 10 }}
          stripe
          noDataElement={<Empty description={tagFilter.length > 0 ? "当前筛选下无任务" : "暂无备份任务，请先点击右上角创建任务"} />}
          rowSelection={writable ? {
            type: 'checkbox',
            selectedRowKeys: selectedIds,
            onChange: (keys) => setSelectedIds(keys.map((k) => Number(k))),
          } : undefined}
        />
      </Card>

      <BackupTaskFormDrawer
        visible={drawerVisible}
        loading={submitting}
        initialValue={editingTask}
        storageTargets={enabledStorageTargets}
        localNodeId={localNodeId}
        nodes={nodes}
        allTasks={tasks.map((t) => ({ id: t.id, name: t.name }))}
        onCancel={() => {
          setDrawerVisible(false)
          setEditingTask(null)
        }}
        onSubmit={handleSubmit}
        onCreateStorageTarget={handleCreateStorageTarget}
        onTestStorageTarget={handleTestStorageTarget}
        onGoogleDriveAuth={handleGoogleDriveAuth}
        onStorageTargetCreated={reloadStorageTargets}
      />

      <BackupTaskDetailDrawer
        visible={detailVisible}
        task={detailTask}
        onCancel={() => {
          setDetailVisible(false)
          setDetailTask(null)
        }}
      />

      <Modal
        visible={importResults !== null}
        title="导入结果"
        footer={null}
        onCancel={() => setImportResults(null)}
        style={{ width: 640 }}
      >
        {importResults && (
          <Table
            rowKey="name"
            pagination={false}
            data={importResults}
            size="small"
            columns={[
              { title: '任务名', dataIndex: 'name' },
              { title: '状态', render: (_: unknown, r: TaskImportResult) => (
                r.skipped ? <Tag color="gray" bordered>跳过</Tag>
                : r.success ? <Tag color="green" bordered>创建</Tag>
                : <Tag color="red" bordered>失败</Tag>
              )},
              { title: 'ID', dataIndex: 'taskId', render: (v?: number) => v ? `#${v}` : '-' },
              { title: '说明', dataIndex: 'error', render: (v?: string) => v || '-' },
            ]}
          />
        )}
      </Modal>
    </Space>
  )
}
