import { Alert, Button, Divider, Drawer, Input, Select, Space, Switch, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { getStorageTargetFieldConfigs, getStorageTargetTypeLabel, storageTargetTypeOptions } from './field-config'
import type { StorageConnectionTestResult, StorageTargetDetail, StorageTargetPayload, StorageTargetType } from '../../types/storage-targets'

interface StorageTargetFormDrawerProps {
  visible: boolean
  loading: boolean
  testing: boolean
  initialValue: StorageTargetDetail | null
  onCancel: () => void
  onSubmit: (value: StorageTargetPayload, targetId?: number) => Promise<void>
  onTest: (value: StorageTargetPayload, targetId?: number) => Promise<StorageConnectionTestResult>
  onGoogleDriveAuth: (value: StorageTargetPayload, targetId?: number) => Promise<void>
}

function createEmptyDraft(type: StorageTargetType = 'local_disk'): StorageTargetPayload {
  return {
    name: '',
    type,
    description: '',
    enabled: true,
    config: {},
  }
}

export function StorageTargetFormDrawer({
  visible,
  loading,
  testing,
  initialValue,
  onCancel,
  onSubmit,
  onTest,
  onGoogleDriveAuth,
}: StorageTargetFormDrawerProps) {
  const [draft, setDraft] = useState<StorageTargetPayload>(createEmptyDraft())
  const [error, setError] = useState('')
  const [testResult, setTestResult] = useState<StorageConnectionTestResult | null>(null)

  useEffect(() => {
    if (!visible) {
      return
    }
    if (!initialValue) {
      setDraft(createEmptyDraft())
      setError('')
      setTestResult(null)
      return
    }
    setDraft({
      name: initialValue.name,
      type: initialValue.type,
      description: initialValue.description,
      enabled: initialValue.enabled,
      config: { ...initialValue.config },
    })
    setError('')
    setTestResult(null)
  }, [initialValue, visible])

  const fieldConfigs = useMemo(() => getStorageTargetFieldConfigs(draft.type), [draft.type])

  function updateConfig(key: string, value: string | boolean) {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        [key]: value,
      },
    }))
  }

  function validate(value: StorageTargetPayload) {
    if (!value.name.trim()) {
      return '请输入存储目标名称'
    }
    for (const field of fieldConfigs) {
      if (!field.required) {
        continue
      }
      const currentValue = value.config[field.key]
      if (field.type === 'switch') {
        continue
      }
      if (typeof currentValue !== 'string' || !currentValue.trim()) {
        return `请填写${field.label}`
      }
    }
    return ''
  }

  async function handleSubmit() {
    const validationError = validate(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    setError('')
    await onSubmit(draft, initialValue?.id)
  }

  async function handleTest() {
    const validationError = validate(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    setError('')
    const result = await onTest(draft, initialValue?.id)
    setTestResult(result)
  }

  async function handleGoogleDriveAuth() {
    const validationError = validate(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    setError('')
    await onGoogleDriveAuth(draft, initialValue?.id)
  }

  return (
    <Drawer
      width={560}
      title={initialValue ? '编辑存储目标' : '新建存储目标'}
      visible={visible}
      onCancel={onCancel}
      unmountOnExit={false}
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {error ? <Alert type="error" content={error} /> : <Alert type="info" content="存储目标提供备份文件的最终去向，请确保服务端网络连通性并通过测试。" />}
        {testResult ? <Alert type={testResult.success ? 'success' : 'warning'} content={testResult.message} /> : null}

        <div>
          <Typography.Text>名称</Typography.Text>
          <Input value={draft.name} placeholder="例如：生产环境 MinIO" onChange={(value) => setDraft((current) => ({ ...current, name: value }))} />
        </div>

        <div>
          <Typography.Text>类型</Typography.Text>
          <Select
            value={draft.type}
            options={storageTargetTypeOptions as unknown as { label: string; value: string }[]}
            onChange={(value) => {
              const nextType = value as StorageTargetType
              setDraft((current) => ({
                ...current,
                type: nextType,
                config: {},
              }))
              setTestResult(null)
            }}
          />
        </div>

        <div>
          <Typography.Text>描述</Typography.Text>
          <Input.TextArea
            value={draft.description}
            placeholder="可选描述，例如备份上传到 NAS 或 Google Drive"
            onChange={(value) => setDraft((current) => ({ ...current, description: value }))}
          />
        </div>

        <Space align="center" size="medium">
          <Typography.Text>启用</Typography.Text>
          <Switch checked={draft.enabled} onChange={(checked) => setDraft((current) => ({ ...current, enabled: checked }))} />
        </Space>

        <Divider orientation="left">环境配置</Divider>

        <div>
          <Typography.Title heading={6} style={{ marginTop: 0, color: 'var(--color-text-2)' }}>
            {getStorageTargetTypeLabel(draft.type)}
          </Typography.Title>
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            {fieldConfigs.map((field) => {
              const value = draft.config[field.key]
              const normalizedValue = typeof value === 'boolean' ? value : typeof value === 'string' ? value : field.type === 'switch' ? false : ''

              return (
                <div key={field.key}>
                  <Typography.Text>
                    {field.label}
                    {field.required ? ' *' : ''}
                  </Typography.Text>
                  {field.type === 'switch' ? (
                    <Space align="center" size="medium">
                      <Switch checked={Boolean(normalizedValue)} onChange={(checked) => updateConfig(field.key, checked)} />
                      {field.description ? <Typography.Text type="secondary">{field.description}</Typography.Text> : null}
                    </Space>
                  ) : field.type === 'password' ? (
                    <Input.Password
                      value={String(normalizedValue)}
                      placeholder={field.placeholder}
                      onChange={(nextValue) => updateConfig(field.key, nextValue)}
                    />
                  ) : (
                    <Input value={String(normalizedValue)} placeholder={field.placeholder} onChange={(nextValue) => updateConfig(field.key, nextValue)} />
                  )}
                  {field.description && field.type !== 'switch' ? (
                    <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
                      {field.description}
                    </Typography.Paragraph>
                  ) : null}
                  {initialValue?.maskedFields?.includes(field.key) && !draft.config[field.key] ? (
                    <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
                      已存在敏感配置，留空则保持不变。
                    </Typography.Paragraph>
                  ) : null}
                </div>
              )
            })}
          </Space>
        </div>

        <Space>
          <Button loading={testing} onClick={handleTest}>
            测试连接
          </Button>
          {draft.type === 'google_drive' ? (
            <Button type="outline" onClick={handleGoogleDriveAuth}>
              {initialValue ? '重新授权 Google Drive' : '发起 Google Drive 授权'}
            </Button>
          ) : null}
          <Button type="primary" loading={loading} onClick={handleSubmit}>
            保存
          </Button>
        </Space>
      </Space>
    </Drawer>
  )
}
