import { Descriptions, Drawer, Space, Tag, Typography } from '@arco-design/web-react'
import type { BackupTaskDetail } from '../../types/backup-tasks'
import { formatDateTime } from '../../utils/format'
import { getBackupTaskStatusColor, getBackupTaskStatusLabel, getBackupTaskTypeLabel } from './field-config'

interface BackupTaskDetailDrawerProps {
  visible: boolean
  task: BackupTaskDetail | null
  onCancel: () => void
}

export function BackupTaskDetailDrawer({ visible, task, onCancel }: BackupTaskDetailDrawerProps) {
  return (
    <Drawer width={560} title="任务详情" visible={visible} onCancel={onCancel}>
      {task ? (
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <div>
            <Typography.Title heading={6} style={{ marginTop: 0, marginBottom: 4 }}>
              {task.name}
            </Typography.Title>
            <Space>
              <Tag color="arcoblue" bordered>{getBackupTaskTypeLabel(task.type)}</Tag>
              <Tag color={task.enabled ? 'green' : 'gray'} bordered>{task.enabled ? '已启用' : '已停用'}</Tag>
              <Tag color={getBackupTaskStatusColor(task.lastStatus)} bordered>{getBackupTaskStatusLabel(task.lastStatus)}</Tag>
            </Space>
          </div>
          <Descriptions
            column={1}
            border
            data={[
              { label: 'Cron', value: task.cronExpr || '仅手动执行' },
              { label: '存储目标', value: task.storageTargetName || task.storageTargetId },
              { label: '保留天数', value: task.retentionDays },
              { label: '最大保留份数', value: task.maxBackups },
              { label: '压缩', value: task.compression },
              { label: '加密', value: task.encrypt ? '已启用' : '未启用' },
              { label: '最近执行', value: formatDateTime(task.lastRunAt) },
              { label: '创建时间', value: formatDateTime(task.createdAt) },
              { label: '更新时间', value: formatDateTime(task.updatedAt) },
            ]}
          />
          {task.type === 'file' ? (
            <Descriptions border column={1} data={[{ label: '源路径', value: task.sourcePath || '-' }, { label: '排除规则', value: task.excludePatterns.join(', ') || '-' }]} />
          ) : null}
          {task.type === 'sqlite' ? <Descriptions border column={1} data={[{ label: 'SQLite 路径', value: task.dbPath || '-' }]} /> : null}
          {task.type === 'mysql' || task.type === 'postgresql' ? (
            <Descriptions
              column={1}
              border
              data={[
                { label: '数据库主机', value: task.dbHost || '-' },
                { label: '数据库端口', value: task.dbPort || '-' },
                { label: '数据库用户', value: task.dbUser || '-' },
                { label: '数据库名称', value: task.dbName || '-' },
                { label: '数据库密码', value: task.maskedFields?.includes('dbPassword') ? '已配置' : '未配置' },
              ]}
            />
          ) : null}
        </Space>
      ) : null}
    </Drawer>
  )
}
