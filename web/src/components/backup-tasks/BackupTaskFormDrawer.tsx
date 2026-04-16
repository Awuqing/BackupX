import { Alert, Button, Divider, Drawer, Input, InputNumber, Select, Space, Steps, Switch, Typography, Grid } from '@arco-design/web-react'
import { IconDelete, IconPlus } from '@arco-design/web-react/icon'
import { useEffect, useMemo, useState } from 'react'
import { CronInput } from '../CronInput'
import type { StorageTargetDetail, StorageTargetPayload, StorageTargetSummary } from '../../types/storage-targets'
import type { StorageConnectionTestResult } from '../../types/storage-targets'
import type { BackupTaskDetail, BackupTaskPayload, BackupTaskType } from '../../types/backup-tasks'
import { DatabasePicker } from '../common/DatabasePicker'
import { DirectoryPicker } from '../common/DirectoryPicker'
import { StorageTargetFormDrawer } from '../storage-targets/StorageTargetFormDrawer'
import {
  backupCompressionOptions,
  backupTaskTypeOptions,
  defaultSapHanaExtraConfig,
  getDefaultPort,
  isDatabaseBackupTask,
  isFileBackupTask,
  isSapHanaBackupTask,
  isSQLiteBackupTask,
  sapHanaBackupLevelOptions,
  sapHanaBackupTypeOptions,
  type SapHanaExtraConfig,
} from './field-config'

interface BackupTaskFormDrawerProps {
  visible: boolean
  loading: boolean
  initialValue: BackupTaskDetail | null
  storageTargets: StorageTargetSummary[]
  localNodeId?: number
  onCancel: () => void
  onSubmit: (value: BackupTaskPayload, taskId?: number) => Promise<void>
  onCreateStorageTarget?: (value: StorageTargetPayload) => Promise<StorageTargetDetail>
  onTestStorageTarget?: (value: StorageTargetPayload, targetId?: number) => Promise<StorageConnectionTestResult>
  onGoogleDriveAuth?: (value: StorageTargetPayload, targetId?: number) => Promise<void>
  onStorageTargetCreated?: () => Promise<void>
}

function createEmptyDraft(storageTargets?: StorageTargetSummary[]): BackupTaskPayload {
  const defaultIds = storageTargets && storageTargets.length > 0 ? [storageTargets[0].id] : []
  return {
    name: '',
    type: 'file',
    enabled: true,
    cronExpr: '',
    sourcePath: '',
    sourcePaths: [''],
    excludePatterns: [],
    dbHost: '',
    dbPort: 0,
    dbUser: '',
    dbPassword: '',
    dbName: '',
    dbPath: '',
    storageTargetId: defaultIds[0] ?? 0,
    storageTargetIds: defaultIds,
    nodeId: 0,
    tags: '',
    retentionDays: 30,
    compression: 'gzip',
    encrypt: false,
    maxBackups: 10,
    extraConfig: undefined,
  }
}

export function BackupTaskFormDrawer({ visible, loading, initialValue, storageTargets, localNodeId, onCancel, onSubmit, onCreateStorageTarget, onTestStorageTarget, onGoogleDriveAuth, onStorageTargetCreated }: BackupTaskFormDrawerProps) {
  const [draft, setDraft] = useState<BackupTaskPayload>(createEmptyDraft())
  const [excludePatternsText, setExcludePatternsText] = useState('')
  const [currentStep, setCurrentStep] = useState(0)
  const [error, setError] = useState('')
  const [quickCreateVisible, setQuickCreateVisible] = useState(false)
  const [quickCreateLoading, setQuickCreateLoading] = useState(false)
  const [quickCreateTesting, setQuickCreateTesting] = useState(false)

  useEffect(() => {
    if (!visible) {
      return
    }

    if (!initialValue) {
      const nextDraft = createEmptyDraft(storageTargets)
      setDraft(nextDraft)
      setExcludePatternsText('')
      setCurrentStep(0)
      setError('')
      return
    }

    const editTargetIds = initialValue.storageTargetIds?.length > 0
      ? initialValue.storageTargetIds
      : initialValue.storageTargetId > 0
        ? [initialValue.storageTargetId]
        : []
    // 编辑时：sourcePaths 优先，为空回退 sourcePath
    const editSourcePaths = initialValue.sourcePaths?.length > 0
      ? initialValue.sourcePaths
      : initialValue.sourcePath
        ? [initialValue.sourcePath]
        : ['']
    setDraft({
      name: initialValue.name,
      type: initialValue.type,
      enabled: initialValue.enabled,
      cronExpr: initialValue.cronExpr,
      sourcePath: initialValue.sourcePath,
      sourcePaths: editSourcePaths,
      excludePatterns: initialValue.excludePatterns,
      dbHost: initialValue.dbHost,
      dbPort: initialValue.dbPort,
      dbUser: initialValue.dbUser,
      dbPassword: '',
      dbName: initialValue.dbName,
      dbPath: initialValue.dbPath,
      storageTargetId: editTargetIds[0] ?? 0,
      storageTargetIds: editTargetIds,
      nodeId: (initialValue as any).nodeId ?? 0,
      tags: (initialValue as any).tags ?? '',
      retentionDays: initialValue.retentionDays,
      compression: initialValue.compression,
      encrypt: initialValue.encrypt,
      maxBackups: initialValue.maxBackups,
      extraConfig: initialValue.extraConfig,
    })
    setExcludePatternsText(initialValue.excludePatterns.join('\n'))
    setCurrentStep(0)
    setError('')
  }, [initialValue, storageTargets, visible])

  const storageTargetOptions = useMemo(
    () => {
      const sorted = [...storageTargets].sort((a, b) => {
        if (a.starred !== b.starred) return a.starred ? -1 : 1
        return 0
      })
      return sorted.map((item) => ({
        label: item.starred ? `★ ${item.name}` : item.name,
        value: item.id,
        disabled: !item.enabled,
      }))
    },
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
      sourcePaths: value === 'file' ? current.sourcePaths : [''],
      excludePatterns: value === 'file' ? current.excludePatterns : [],
      dbHost: value === 'mysql' || value === 'postgresql' || value === 'saphana' ? current.dbHost : '',
      dbPort: value === 'mysql' || value === 'postgresql' || value === 'saphana' ? current.dbPort || getDefaultPort(value) : 0,
      dbUser: value === 'mysql' || value === 'postgresql' || value === 'saphana' ? current.dbUser : '',
      dbPassword: value === 'mysql' || value === 'postgresql' || value === 'saphana' ? current.dbPassword : '',
      dbName: value === 'mysql' || value === 'postgresql' || value === 'saphana' ? current.dbName : '',
      dbPath: value === 'sqlite' ? current.dbPath : '',
      // 切换到 SAP HANA 时初始化扩展配置；切换到其他类型时清空
      extraConfig: value === 'saphana'
        ? ({ ...defaultSapHanaExtraConfig(), ...(current.extraConfig as SapHanaExtraConfig | undefined) } as unknown as Record<string, unknown>)
        : undefined,
    }))
    if (value !== 'file') {
      setExcludePatternsText('')
    }
  }

  // 更新 SAP HANA 扩展配置的辅助函数
  function updateHanaExtraConfig(patch: Partial<SapHanaExtraConfig>) {
    setDraft((current) => {
      const merged: SapHanaExtraConfig = {
        ...defaultSapHanaExtraConfig(),
        ...(current.extraConfig as SapHanaExtraConfig | undefined),
        ...patch,
      }
      return { ...current, extraConfig: merged as unknown as Record<string, unknown> }
    })
  }

  function validate(value: BackupTaskPayload) {
    if (!value.name.trim()) {
      return '请输入任务名称'
    }
    if (!value.storageTargetIds || value.storageTargetIds.length === 0) {
      return '请选择至少一个存储目标'
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
    if (isFileBackupTask(value.type)) {
      const validPaths = (value.sourcePaths ?? []).filter((p) => p.trim())
      if (validPaths.length === 0 && !value.sourcePath.trim()) {
        return '请输入至少一个源路径'
      }
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
    const validSourcePaths = (draft.sourcePaths ?? []).filter((p) => p.trim())
    const nextValue: BackupTaskPayload = {
      ...draft,
      sourcePaths: validSourcePaths,
      sourcePath: validSourcePaths[0] ?? draft.sourcePath,
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

  function updateSourcePath(index: number, value: string) {
    setDraft((current) => {
      const next = [...(current.sourcePaths ?? [''])]
      next[index] = value
      return { ...current, sourcePaths: next, sourcePath: next[0] ?? '' }
    })
  }

  function addSourcePath() {
    setDraft((current) => ({
      ...current,
      sourcePaths: [...(current.sourcePaths ?? ['']), ''],
    }))
  }

  function removeSourcePath(index: number) {
    setDraft((current) => {
      const next = [...(current.sourcePaths ?? [''])]
      next.splice(index, 1)
      if (next.length === 0) next.push('')
      return { ...current, sourcePaths: next, sourcePath: next[0] ?? '' }
    })
  }

  function renderSourceStep() {
    const paths = draft.sourcePaths?.length > 0 ? draft.sourcePaths : ['']
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {isFileBackupTask(draft.type) ? (
          <>
            <div>
              <Typography.Text>源路径</Typography.Text>
              <Space direction="vertical" size="medium" style={{ width: '100%' }}>
                {paths.map((p, index) => (
                  <Grid.Row key={index} gutter={8} align="center">
                    <Grid.Col flex="auto">
                      <DirectoryPicker
                        value={p}
                        placeholder={`源路径 ${index + 1}，例如：/var/www/html`}
                        mode="directory"
                        nodeId={localNodeId}
                        onChange={(value) => updateSourcePath(index, value)}
                      />
                    </Grid.Col>
                    <Grid.Col flex="none">
                      <Button
                        type="text"
                        icon={<IconDelete />}
                        status="danger"
                        disabled={paths.length <= 1}
                        onClick={() => removeSourcePath(index)}
                      />
                    </Grid.Col>
                  </Grid.Row>
                ))}
                <Button type="dashed" long icon={<IconPlus />} onClick={addSourcePath}>
                  添加源路径
                </Button>
              </Space>
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
            <DirectoryPicker
              value={draft.dbPath}
              placeholder="例如：/data/app.db"
              mode="file"
              nodeId={localNodeId}
              onChange={(value) => updateDraft({ dbPath: value })}
            />
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
              {(draft.type === 'mysql' || draft.type === 'postgresql') ? (
                <DatabasePicker
                  dbType={draft.type}
                  dbHost={draft.dbHost}
                  dbPort={draft.dbPort}
                  dbUser={draft.dbUser}
                  dbPassword={draft.dbPassword}
                  value={draft.dbName}
                  onChange={(value) => updateDraft({ dbName: value })}
                />
              ) : (
                <Input value={draft.dbName} placeholder="例如：app_prod" onChange={(value) => updateDraft({ dbName: value })} />
              )}
            </div>
            {isSapHanaBackupTask(draft.type) ? renderSapHanaExtraFields() : null}
          </>
        ) : null}
      </Space>
    )
  }

  function renderSapHanaExtraFields() {
    const hana: SapHanaExtraConfig = {
      ...defaultSapHanaExtraConfig(),
      ...(draft.extraConfig as SapHanaExtraConfig | undefined),
    }
    return (
      <>
        <Divider style={{ margin: '8px 0' }} orientation="left">
          <Typography.Text type="secondary">SAP HANA 扩展配置</Typography.Text>
        </Divider>
        <div>
          <Typography.Text>备份类型</Typography.Text>
          <Select
            style={{ width: '100%' }}
            value={hana.backupType}
            options={[...sapHanaBackupTypeOptions]}
            onChange={(value) => updateHanaExtraConfig({ backupType: value as SapHanaExtraConfig['backupType'] })}
          />
        </div>
        <div>
          <Typography.Text>备份级别</Typography.Text>
          <Select
            style={{ width: '100%' }}
            value={hana.backupLevel}
            options={[...sapHanaBackupLevelOptions]}
            disabled={hana.backupType === 'log'}
            onChange={(value) => updateHanaExtraConfig({ backupLevel: value as SapHanaExtraConfig['backupLevel'] })}
          />
          {hana.backupType === 'log' ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>日志备份不支持级别配置</Typography.Text>
          ) : null}
        </div>
        <div>
          <Typography.Text>并行通道数</Typography.Text>
          <InputNumber
            style={{ width: '100%' }}
            value={hana.backupChannels}
            min={1}
            max={32}
            onChange={(value) => updateHanaExtraConfig({ backupChannels: Number(value ?? 1) })}
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{'>'} 1 时启用 HANA 多路径并行备份</Typography.Text>
        </div>
        <div>
          <Typography.Text>失败重试次数</Typography.Text>
          <InputNumber
            style={{ width: '100%' }}
            value={hana.maxRetries}
            min={1}
            max={10}
            onChange={(value) => updateHanaExtraConfig({ maxRetries: Number(value ?? 3) })}
          />
        </div>
        <div>
          <Typography.Text>实例编号（可选）</Typography.Text>
          <Input
            value={hana.instanceNumber}
            placeholder="留空将根据端口自动推断（例如 30015 → 0）"
            onChange={(value) => updateHanaExtraConfig({ instanceNumber: value })}
          />
        </div>
      </>
    )
  }

  async function handleQuickCreateSubmit(value: StorageTargetPayload) {
    if (!onCreateStorageTarget) return
    setQuickCreateLoading(true)
    try {
      const created = await onCreateStorageTarget(value)
      setQuickCreateVisible(false)
      if (onStorageTargetCreated) {
        await onStorageTargetCreated()
      }
      const currentIds = draft.storageTargetIds ?? []
      const nextIds = [...currentIds, created.id]
      updateDraft({ storageTargetIds: nextIds, storageTargetId: nextIds[0] ?? 0 })
    } finally {
      setQuickCreateLoading(false)
    }
  }

  async function handleQuickCreateTest(value: StorageTargetPayload, targetId?: number): Promise<StorageConnectionTestResult> {
    if (!onTestStorageTarget) return { success: false, message: '测试不可用' }
    setQuickCreateTesting(true)
    try {
      return await onTestStorageTarget(value, targetId)
    } finally {
      setQuickCreateTesting(false)
    }
  }

  function renderPolicyStep() {
    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Text>存储目标</Typography.Text>
          <Space style={{ width: '100%' }} align="start">
            <Select
              style={{ flex: 1 }}
              mode="multiple"
              value={draft.storageTargetIds?.length > 0 ? draft.storageTargetIds : undefined}
              placeholder="请选择存储目标（可多选）"
              options={storageTargetOptions}
              onChange={(values: number[]) => updateDraft({ storageTargetIds: values, storageTargetId: values[0] ?? 0 })}
            />
            {onCreateStorageTarget && (
              <Button type="outline" size="small" onClick={() => setQuickCreateVisible(true)}>
                + 快速新建
              </Button>
            )}
          </Space>
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

      {onCreateStorageTarget && (
        <StorageTargetFormDrawer
          visible={quickCreateVisible}
          loading={quickCreateLoading}
          testing={quickCreateTesting}
          initialValue={null}
          onCancel={() => setQuickCreateVisible(false)}
          onSubmit={handleQuickCreateSubmit}
          onTest={handleQuickCreateTest}
          onGoogleDriveAuth={onGoogleDriveAuth ?? (async () => {})}
        />
      )}
    </Drawer>
  )
}
