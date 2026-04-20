import { Alert, Descriptions, Modal, Space, Tag, Typography } from '@arco-design/web-react'
import type { BackupRecordDetail } from '../../types/backup-records'
import type { BackupTaskDetail } from '../../types/backup-tasks'

interface RestoreConfirmModalProps {
  visible: boolean
  loading: boolean
  backupRecord: BackupRecordDetail | null
  task: BackupTaskDetail | null
  onCancel: () => void
  onConfirm: () => void
}

// RestoreConfirmModal 展示即将恢复的备份摘要与覆盖风险，强制用户二次确认。
// 恢复是破坏性操作：会覆盖任务配置的源路径/数据库，不可撤销。
export function RestoreConfirmModal({ visible, loading, backupRecord, task, onCancel, onConfirm }: RestoreConfirmModalProps) {
  if (!backupRecord || !task) {
    return (
      <Modal visible={visible} title="确认恢复" onCancel={onCancel} onOk={onConfirm} confirmLoading={loading} unmountOnExit>
        <Alert type="info" content="正在加载任务与备份信息..." />
      </Modal>
    )
  }

  const restoreTarget = renderRestoreTarget(task)
  const nodeLabel = task.nodeId && task.nodeId > 0
    ? (task.nodeName ? `${task.nodeName}（远程节点）` : `节点 #${task.nodeId}`)
    : '本机 Master'

  return (
    <Modal
      visible={visible}
      title="确认执行恢复"
      okText="开始恢复"
      cancelText="取消"
      okButtonProps={{ status: 'danger', loading }}
      onCancel={onCancel}
      onOk={onConfirm}
      unmountOnExit
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
            { label: '类型', value: <Tag color="arcoblue" bordered>{task.type.toUpperCase()}</Tag> },
            { label: '执行节点', value: nodeLabel },
            { label: '源备份', value: backupRecord.fileName || '-' },
            { label: '恢复目标', value: restoreTarget },
          ]}
        />
      </Space>
    </Modal>
  )
}

function renderRestoreTarget(task: BackupTaskDetail) {
  if (task.type === 'file') {
    const paths = task.sourcePaths && task.sourcePaths.length > 0
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
          <Typography.Text key={p} code>{p}</Typography.Text>
        ))}
      </Space>
    )
  }
  if (task.type === 'sqlite') {
    return <Typography.Text code>{task.dbPath || '-'}</Typography.Text>
  }
  if (task.type === 'mysql' || task.type === 'postgresql' || task.type === 'saphana') {
    return (
      <Typography.Text>
        {task.dbUser}@{task.dbHost}:{task.dbPort} / <Typography.Text code>{task.dbName || '-'}</Typography.Text>
      </Typography.Text>
    )
  }
  return '-'
}
