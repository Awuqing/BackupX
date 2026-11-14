import { Alert, Descriptions, Input, Modal, Space, Tag, Typography } from '@arco-design/web-react'
import { useState } from 'react'
import type { BackupRecordDetail } from '../../types/backup-records'
import type { BackupTaskDetail } from '../../types/backup-tasks'

interface RestoreConfirmModalProps {
  visible: boolean
  loading: boolean
  backupRecord: BackupRecordDetail | null
  task: BackupTaskDetail | null
  onCancel: () => void
  onConfirm: (targetPath?: string, restoreTime?: string) => void
}

// RestoreConfirmModal 展示即将恢复的备份摘要与覆盖风险，强制用户二次确认。
// 恢复是破坏性操作：会覆盖任务配置的源路径/数据库，不可撤销。
// 文件类型 + 本机恢复时，允许指定「恢复到其他目录」以避免覆盖原位置。
export function RestoreConfirmModal({
  visible,
  loading,
  backupRecord,
  task,
  onCancel,
  onConfirm,
}: RestoreConfirmModalProps) {
  const [targetPath, setTargetPath] = useState('')
  const [restoreTime, setRestoreTime] = useState('')

  if (!backupRecord || !task) {
    return (
      <Modal
        visible={visible}
        title="确认恢复"
        onCancel={onCancel}
        onOk={() => onConfirm()}
        confirmLoading={loading}
        unmountOnExit
      >
        <Alert type="info" content="正在加载任务与备份信息..." />
      </Modal>
    )
  }

  const checkpoint = backupRecord.mysqlCheckpoint
  const allowPitr = task.type === 'mysql' && backupRecord.backupKind === 'binlog' && checkpoint
  const requested = restoreTime.trim() ? Date.parse(restoreTime) : null
  const invalidTime = Boolean(
    allowPitr &&
    requested !== null &&
    (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
      restoreTime.trim(),
    ) ||
      !Number.isFinite(requested) ||
      requested < Date.parse(checkpoint.baselineAt) ||
      requested >= Date.parse(checkpoint.capturedAt)),
  )
  const restoreTarget = renderRestoreTarget(task)
  const isLocal = !task.nodeId || task.nodeId === 0
  const allowAltPath = task.type === 'file' && isLocal
  const nodeLabel =
    task.nodeId && task.nodeId > 0
      ? task.nodeName
        ? `${task.nodeName}（远程节点）`
        : `节点 #${task.nodeId}`
      : '本机 Master'

  return (
    <Modal
      visible={visible}
      title="确认执行恢复"
      okText="开始恢复"
      cancelText="取消"
      okButtonProps={{ status: 'danger', loading, disabled: invalidTime }}
      onCancel={onCancel}
      onOk={() => {
        if (!invalidTime) {
          onConfirm(
            targetPath.trim() || undefined,
            allowPitr && requested !== null ? new Date(requested).toISOString() : undefined,
          )
        }
      }}
      unmountOnExit
      afterClose={() => {
        setTargetPath('')
        setRestoreTime('')
      }}
    >
      <Space direction="vertical" size="medium" style={{ width: '100%' }}>
        <Alert
          type="warning"
          content="恢复是破坏性操作：会覆盖目标位置的现有数据，不可撤销。请先确认恢复目标并在必要时保留当前状态的副本。"
        />
        <Descriptions
          column={1}
          size="small"
          data={[
            { label: '任务', value: <Typography.Text bold>{task.name}</Typography.Text> },
            {
              label: '类型',
              value: (
                <Tag color="arcoblue" bordered>
                  {task.type.toUpperCase()}
                </Tag>
              ),
            },
            { label: '执行节点', value: nodeLabel },
            { label: '源备份', value: backupRecord.fileName || '-' },
            {
              label: '恢复目标',
              value: targetPath.trim() ? (
                <Typography.Text code>{targetPath.trim()}</Typography.Text>
              ) : (
                restoreTarget
              ),
            },
          ]}
        />
        {allowPitr && (
          <div>
            <Typography.Text>
              恢复时间（可选，带时区的 RFC3339；留空恢复至日志截止位置）
            </Typography.Text>
            <Input
              value={restoreTime}
              placeholder="2026-11-14T12:30:00+08:00"
              onChange={setRestoreTime}
              error={invalidTime}
              allowClear
              aria-label="MySQL 恢复时间"
            />
            <Typography.Paragraph
              type={invalidTime ? 'error' : 'secondary'}
              style={{ marginTop: 4 }}
            >
              可选范围：{new Date(checkpoint.baselineAt).toLocaleString()} 至{' '}
              {new Date(checkpoint.capturedAt).toLocaleString()}
              （不含截止秒）。仅填写带时区的时间；恢复覆盖整个实例，建议先在隔离实例演练。
            </Typography.Paragraph>
          </div>
        )}
        {allowAltPath && (
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              恢复到指定目录（可选，绝对路径；留空则恢复到原始位置）
            </Typography.Text>
            <Input
              style={{ marginTop: 4 }}
              allowClear
              value={targetPath}
              placeholder="/path/to/restore-here"
              onChange={(value) => setTargetPath(value)}
            />
          </div>
        )}
      </Space>
    </Modal>
  )
}

function renderRestoreTarget(task: BackupTaskDetail) {
  if (task.type === 'file') {
    const paths =
      task.sourcePaths && task.sourcePaths.length > 0
        ? task.sourcePaths
        : task.sourcePath
          ? [task.sourcePath]
          : []
    if (paths.length === 0) {
      return <Typography.Text type="secondary">未配置源路径</Typography.Text>
    }
    return (
      <Space direction="vertical" size={2}>
        {paths.map((p) => (
          <Typography.Text key={p} code>
            {p}
          </Typography.Text>
        ))}
      </Space>
    )
  }
  if (task.type === 'sqlite') {
    return <Typography.Text code>{task.dbPath || '-'}</Typography.Text>
  }
  if (
    task.type === 'mysql' ||
    task.type === 'postgresql' ||
    task.type === 'saphana' ||
    task.type === 'mongodb'
  ) {
    return (
      <Typography.Text>
        {task.dbUser}@{task.dbHost}:{task.dbPort} /{' '}
        <Typography.Text code>{task.dbName || '-'}</Typography.Text>
      </Typography.Text>
    )
  }
  return '-'
}
