import { Alert, Button, Drawer, Input, InputNumber, Select, Space, Switch, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import type { NotificationDetail, NotificationPayload, NotificationType } from '../../types/notifications'
import { getNotificationFieldConfigs, getNotificationTypeLabel, notificationTypeOptions } from './field-config'

interface NotificationFormDrawerProps {
  visible: boolean
  loading: boolean
  testing: boolean
  initialValue: NotificationDetail | null
  onCancel: () => void
  onSubmit: (value: NotificationPayload, notificationId?: number) => Promise<void>
  onTest: (value: NotificationPayload, notificationId?: number) => Promise<void>
}

function createEmptyDraft(): NotificationPayload {
  return {
    name: '',
    type: 'webhook',
    enabled: true,
    onSuccess: false,
    onFailure: true,
    config: {},
  }
}

export function NotificationFormDrawer({ visible, loading, testing, initialValue, onCancel, onSubmit, onTest }: NotificationFormDrawerProps) {
  const [draft, setDraft] = useState<NotificationPayload>(createEmptyDraft())
  const [error, setError] = useState('')

  useEffect(() => {
    if (!visible) {
      return
    }
    if (!initialValue) {
      setDraft(createEmptyDraft())
      setError('')
      return
    }
    setDraft({
      name: initialValue.name,
      type: initialValue.type,
      enabled: initialValue.enabled,
      onSuccess: initialValue.onSuccess,
      onFailure: initialValue.onFailure,
      config: { ...initialValue.config },
    })
    setError('')
  }, [initialValue, visible])

  const fieldConfigs = useMemo(() => getNotificationFieldConfigs(draft.type), [draft.type])

  function updateDraft(patch: Partial<NotificationPayload>) {
    setDraft((current) => ({ ...current, ...patch }))
  }

  function updateConfig(key: string, value: string | number) {
    setDraft((current) => ({
      ...current,
      config: {
        ...current.config,
        [key]: value,
      },
    }))
  }

  function validate(value: NotificationPayload) {
    if (!value.name.trim()) {
      return '请输入通知名称'
    }
    for (const field of fieldConfigs) {
      if (!field.required) {
        continue
      }
      const currentValue = value.config[field.key]
      if (typeof currentValue === 'number' && currentValue > 0) {
        continue
      }
      if (typeof currentValue === 'string' && currentValue.trim()) {
        continue
      }
      if (initialValue?.maskedFields?.includes(field.key) && (currentValue === '' || currentValue === undefined)) {
        continue
      }
      return `请填写${field.label}`
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
    await onTest(draft, initialValue?.id)
  }

  return (
    <Drawer width={560} title={initialValue ? '编辑通知配置' : '新建通知配置'} visible={visible} onCancel={onCancel} unmountOnExit={false}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {error ? <Alert type="error" content={error} /> : null}
        <div>
          <Typography.Text>名称</Typography.Text>
          <Input value={draft.name} placeholder="例如：生产故障通知" onChange={(value) => updateDraft({ name: value })} />
        </div>
        <div>
          <Typography.Text>类型</Typography.Text>
          <Select value={draft.type} options={notificationTypeOptions as unknown as { label: string; value: string }[]} onChange={(value) => updateDraft({ type: value as NotificationType, config: {} })} />
        </div>
        <Space align="center" size="medium">
          <Typography.Text>启用</Typography.Text>
          <Switch checked={draft.enabled} onChange={(checked) => updateDraft({ enabled: checked })} />
        </Space>
        <Space align="center" size="medium">
          <Typography.Text>成功时通知</Typography.Text>
          <Switch checked={draft.onSuccess} onChange={(checked) => updateDraft({ onSuccess: checked })} />
        </Space>
        <Space align="center" size="medium">
          <Typography.Text>失败时通知</Typography.Text>
          <Switch checked={draft.onFailure} onChange={(checked) => updateDraft({ onFailure: checked })} />
        </Space>
        <div>
          <Typography.Title heading={6} style={{ marginTop: 0 }}>
            {getNotificationTypeLabel(draft.type)} 配置
          </Typography.Title>
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            {fieldConfigs.map((field) => {
              const currentValue = draft.config[field.key]
              const normalizedValue = typeof currentValue === 'number' || typeof currentValue === 'string' ? currentValue : field.type === 'number' ? 0 : ''

              return (
                <div key={field.key}>
                  <Typography.Text>
                    {field.label}
                    {field.required ? ' *' : ''}
                  </Typography.Text>
                  {field.type === 'password' ? (
                    <Input.Password value={String(normalizedValue)} placeholder={field.placeholder} onChange={(value) => updateConfig(field.key, value)} />
                  ) : field.type === 'number' ? (
                    <InputNumber style={{ width: '100%' }} value={Number(normalizedValue)} min={0} onChange={(value) => updateConfig(field.key, Number(value ?? 0))} />
                  ) : field.type === 'textarea' ? (
                    <Input.TextArea value={String(normalizedValue)} placeholder={field.placeholder} onChange={(value) => updateConfig(field.key, value)} />
                  ) : (
                    <Input value={String(normalizedValue)} placeholder={field.placeholder} onChange={(value) => updateConfig(field.key, value)} />
                  )}
                  {field.description ? (
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
            发送测试通知
          </Button>
          <Button type="primary" loading={loading} onClick={handleSubmit}>
            保存配置
          </Button>
        </Space>
      </Space>
    </Drawer>
  )
}
