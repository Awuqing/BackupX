import { Alert, Button, Divider, Drawer, Input, InputNumber, Select, Space, Steps, Switch, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { CronInput } from '../CronInput'
import type { StorageTargetSummary } from '../../types/storage-targets'
import type { BackupTaskDetail, BackupTaskPayload, BackupTaskType } from '../../types/backup-tasks'
import {
  backupCompressionOptions,
  backupTaskTypeOptions,
  getDefaultPort,
  isDatabaseBackupTask,
  isFileBackupTask,
  isSQLiteBackupTask,
} from './field-config'

interface BackupTaskFormDrawerProps {
  visible: boolean
  loading: boolean
  initialValue: BackupTaskDetail | null
  storageTargets: StorageTargetSummary[]
  onCancel: () => void
  onSubmit: (value: BackupTaskPayload, taskId?: number) => Promise<void>
}

function createEmptyDraft(storageTargetId?: number): BackupTaskPayload {
  return {
    name: '',
    type: 'file',
    enabled: true,
    cronExpr: '',
    sourcePath: '',
    excludePatterns: [],
    dbHost: '',
    dbPort: 0,
    dbUser: '',
    dbPassword: '',
    dbName: '',
    dbPath: '',
    storageTargetId: storageTargetId ?? 0,
    nodeId: 0,
    tags: '',
    retentionDays: 30,
    compression: 'gzip',
    encrypt: false,
    maxBackups: 10,
  }
}

export function BackupTaskFormDrawer({ visible, loading, initialValue, storageTargets, onCancel, onSubmit }: BackupTaskFormDrawerProps) {
  const [draft, setDraft] = useState<BackupTaskPayload>(createEmptyDraft())
  const [excludePatternsText, setExcludePatternsText] = useState('')
  const [currentStep, setCurrentStep] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!visible) {
      return
    }

    if (!initialValue) {
      const nextDraft = createEmptyDraft(storageTargets[0]?.id)
      setDraft(nextDraft)
      setExcludePatternsText('')
      setCurrentStep(0)
      setError('')
      return
    }

    setDraft({
      name: initialValue.name,
      type: initialValue.type,
      enabled: initialValue.enabled,
      cronExpr: initialValue.cronExpr,
      sourcePath: initialValue.sourcePath,
      excludePatterns: initialValue.excludePatterns,
      dbHost: initialValue.dbHost,
      dbPort: initialValue.dbPort,
      dbUser: initialValue.dbUser,
      dbPassword: '',
      dbName: initialValue.dbName,
      dbPath: initialValue.dbPath,
      storageTargetId: initialValue.storageTargetId,
      nodeId: (initialValue as any).nodeId ?? 0,
      tags: (initialValue as any).tags ?? '',
      retentionDays: initialValue.retentionDays,
      compression: initialValue.compression,
      encrypt: initialValue.encrypt,
      maxBackups: initialValue.maxBackups,
    })
    setExcludePatternsText(initialValue.excludePatterns.join('\n'))
    setCurrentStep(0)
    setError('')
  }, [initialValue, storageTargets, visible])

  const storageTargetOptions = useMemo(
    () => storageTargets.map((item) => ({ label: item.name, value: item.id, disabled: !item.enabled })),
    [storageTargets],
  )

  function updateDraft(patch: Partial<BackupTaskPayload>) {
    setDraft((current) => ({ ...current, ...patch }))
  }

  function updateTaskType(value: BackupTaskType) {
    setDraft((current) => ({
      ...current,
      type: value,
      sourcePath: value === 'file' ? current.sourcePath : '',
      excludePatterns: value === 'file' ? current.excludePatterns : [],
      dbHost: value === 'mysql' || value === 'postgresql' ? current.dbHost : '',
      dbPort: value === 'mysql' || value === 'postgresql' ? current.dbPort || getDefaultPort(value) : 0,
      dbUser: value === 'mysql' || value === 'postgresql' ? current.dbUser : '',
      dbPassword: value === 'mysql' || value === 'postgresql' ? current.dbPassword : '',
      dbName: value === 'mysql' || value === 'postgresql' ? current.dbName : '',
      dbPath: value === 'sqlite' ? current.dbPath : '',
    }))
    if (value !== 'file') {
      setExcludePatternsText('')
    }
  }

  function validate(value: BackupTaskPayload) {
    if (!value.name.trim()) {
      return '请输入任务名称'
    }
    if (!value.storageTargetId) {
      return '请选择存储目标'
    }
    if (value.cronExpr.trim() && value.cronExpr.trim().split(/\s+/).length < 5) {
      return 'Cron 表达式至少需要 5 段'
    }
    if (value.retentionDays < 0) {
      return '保留天数不能小于 0'
    }
    if (value.maxBackups < 0) {
      return '最大保留份数不能小于 0'
    }
    if (isFileBackupTask(value.type) && !value.sourcePath.trim()) {
      return '请输入源路径'
    }
    if (isSQLiteBackupTask(value.type) && !value.dbPath.trim()) {
      return '请输入 SQLite 数据库路径'
    }
    if (isDatabaseBackupTask(value.type)) {
      if (!value.dbHost.trim()) {
        return '请输入数据库主机'
      }
      if (!value.dbPort || value.dbPort <= 0) {
        return '请输入正确的数据库端口'
      }
      if (!value.dbUser.trim()) {
        return '请输入数据库用户名'
      }
      if (!initialValue?.maskedFields?.includes('dbPassword') && !value.dbPassword.trim()) {
        return '请输入数据库密码'
      }
      if (!value.dbName.trim()) {
        return '请输入数据库名称'
      }
    }
    return ''
  }

  async function handleSubmit() {
    const nextValue: BackupTaskPayload = {
      ...draft,
      excludePatterns: excludePatternsText
        .split('\n')
        .map((item) => item.trim())
        .filter(Boolean),
    }
    const validationError = validate(nextValue)
    if (validationError) {
      setError(validationError)
      return
    }
    setError('')
    await onSubmit(nextValue, initialValue?.id)
  }

  function renderBasicStep() {
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Text>任务名称</Typography.Text>
          <Input value={draft.name} placeholder="例如：生产站点每日备份" onChange={(value) => updateDraft({ name: value })} />
        </div>
        <div>
          <Typography.Text>备份类型</Typography.Text>
          <Select value={draft.type} options={backupTaskTypeOptions as unknown as { label: string; value: string }[]} onChange={(value) => updateTaskType(value as BackupTaskType)} />
        </div>
        <div>
          <Typography.Text>Cron 表达式</Typography.Text>
          <CronInput value={draft.cronExpr} onChange={(value) => updateDraft({ cronExpr: value })} />
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
            留空表示仅手动执行；已填写时由服务端调度器自动触发。
          </Typography.Paragraph>
        </div>
        <Space align="center" size="medium">
          <Typography.Text>启用任务</Typography.Text>
          <Switch checked={draft.enabled} onChange={(checked) => updateDraft({ enabled: checked })} />
        </Space>
      </Space>
    )
  }

  function renderSourceStep() {
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {isFileBackupTask(draft.type) ? (
          <>
            <div>
              <Typography.Text>源路径</Typography.Text>
              <Input value={draft.sourcePath} placeholder="例如：/var/www/html" onChange={(value) => updateDraft({ sourcePath: value })} />
            </div>
            <div>
              <Typography.Text>排除规则</Typography.Text>
              <Input.TextArea
                value={excludePatternsText}
                placeholder="每行一条，例如：node_modules\n*.log"
                autoSize={{ minRows: 4, maxRows: 8 }}
                onChange={(value) => setExcludePatternsText(value)}
              />
            </div>
          </>
        ) : null}

        {isSQLiteBackupTask(draft.type) ? (
          <div>
            <Typography.Text>SQLite 数据库文件</Typography.Text>
            <Input value={draft.dbPath} placeholder="例如：/data/app.db" onChange={(value) => updateDraft({ dbPath: value })} />
          </div>
        ) : null}

        {isDatabaseBackupTask(draft.type) ? (
          <>
            <div>
              <Typography.Text>数据库主机</Typography.Text>
              <Input value={draft.dbHost} placeholder="例如：127.0.0.1" onChange={(value) => updateDraft({ dbHost: value })} />
            </div>
            <div>
              <Typography.Text>数据库端口</Typography.Text>
              <InputNumber style={{ width: '100%' }} value={draft.dbPort} min={1} onChange={(value) => updateDraft({ dbPort: Number(value ?? 0) })} />
            </div>
            <div>
              <Typography.Text>数据库用户名</Typography.Text>
              <Input value={draft.dbUser} placeholder="例如：backup" onChange={(value) => updateDraft({ dbUser: value })} />
            </div>
            <div>
              <Typography.Text>数据库密码</Typography.Text>
              <Input.Password value={draft.dbPassword} placeholder={initialValue?.maskedFields?.includes('dbPassword') ? '留空表示保持原密码' : '请输入数据库密码'} onChange={(value) => updateDraft({ dbPassword: value })} />
            </div>
            <div>
              <Typography.Text>数据库名称</Typography.Text>
              <Input value={draft.dbName} placeholder="例如：app_prod" onChange={(value) => updateDraft({ dbName: value })} />
            </div>
          </>
        ) : null}
      </Space>
    )
  }

  function renderPolicyStep() {
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Text>存储目标</Typography.Text>
          <Select value={draft.storageTargetId || undefined} placeholder="请选择存储目标" options={storageTargetOptions} onChange={(value) => updateDraft({ storageTargetId: Number(value) })} />
        </div>
        <div>
          <Typography.Text>压缩策略</Typography.Text>
          <Select value={draft.compression} options={backupCompressionOptions as unknown as { label: string; value: string }[]} onChange={(value) => updateDraft({ compression: value as BackupTaskPayload['compression'] })} />
        </div>
        <div>
          <Typography.Text>保留天数</Typography.Text>
          <InputNumber style={{ width: '100%' }} value={draft.retentionDays} min={0} onChange={(value) => updateDraft({ retentionDays: Number(value ?? 0) })} />
        </div>
        <div>
          <Typography.Text>最大保留份数</Typography.Text>
          <InputNumber style={{ width: '100%' }} value={draft.maxBackups} min={0} onChange={(value) => updateDraft({ maxBackups: Number(value ?? 0) })} />
        </div>
        <Space align="center" size="medium">
          <Typography.Text>备份后加密</Typography.Text>
          <Switch checked={draft.encrypt} onChange={(checked) => updateDraft({ encrypt: checked })} />
        </Space>
      </Space>
    )
  }

  return (
    <Drawer
      width={640}
      title={initialValue ? '编辑备份任务' : '新建备份任务'}
      visible={visible}
      onCancel={onCancel}
      unmountOnExit={false}
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {error ? <Alert type="error" content={error} /> : <Alert type="info" content="配置数据库或文件的自动备份任务，系统将按策略执行并自动清理过期份数。" />}
        <Steps current={currentStep} size="small">
          <Steps.Step title="基础信息" />
          <Steps.Step title="源配置" />
          <Steps.Step title="存储与策略" />
        </Steps>
        <Divider style={{ margin: 0 }} />
        {currentStep === 0 ? renderBasicStep() : null}
        {currentStep === 1 ? renderSourceStep() : null}
        {currentStep === 2 ? renderPolicyStep() : null}
        <Space>
          <Button disabled={currentStep === 0} onClick={() => setCurrentStep((value) => Math.max(0, value - 1))}>
            上一步
          </Button>
          {currentStep < 2 ? (
            <Button type="outline" onClick={() => setCurrentStep((value) => Math.min(2, value + 1))}>
              下一步
            </Button>
          ) : (
            <Button type="primary" loading={loading} onClick={handleSubmit}>
              保存任务
            </Button>
          )}
        </Space>
      </Space>
    </Drawer>
  )
}
