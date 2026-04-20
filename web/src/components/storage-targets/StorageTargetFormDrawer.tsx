import { Alert, Button, Collapse, Divider, Drawer, Input, InputNumber, Select, Space, Switch, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { getStorageTargetFieldConfigs, getStorageTargetTypeLabel, isBuiltinType, buildAllTypeOptions } from './field-config'
import type { StorageConnectionTestResult, StorageTargetDetail, StorageTargetPayload, StorageTargetType } from '../../types/storage-targets'
import { listRcloneBackends, type RcloneBackendInfo, type RcloneBackendOption } from '../../services/rclone'

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
  return { name: '', type, description: '', enabled: true, config: {}, quotaBytes: 0 }
}

export function StorageTargetFormDrawer({
  visible, loading, testing, initialValue, onCancel, onSubmit, onTest, onGoogleDriveAuth,
}: StorageTargetFormDrawerProps) {
  const [draft, setDraft] = useState<StorageTargetPayload>(createEmptyDraft())
  const [error, setError] = useState('')
  const [testResult, setTestResult] = useState<StorageConnectionTestResult | null>(null)
  const [rcloneBackends, setRcloneBackends] = useState<RcloneBackendInfo[]>([])
  const [backendsLoaded, setBackendsLoaded] = useState(false)

  // 加载 rclone 后端列表
  useEffect(() => {
    if (visible && !backendsLoaded) {
      listRcloneBackends()
        .then((data) => { setRcloneBackends(data); setBackendsLoaded(true) })
        .catch(() => setBackendsLoaded(true))
    }
  }, [visible, backendsLoaded])

  useEffect(() => {
    if (!visible) return
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
      quotaBytes: initialValue.quotaBytes ?? 0,
    })
    setError('')
    setTestResult(null)
  }, [initialValue, visible])

  // 构建分类的类型选项（去重、中文标注）
  const allTypeOptions = useMemo(() => buildAllTypeOptions(rcloneBackends), [rcloneBackends])

  // 按分组聚合，用于 Select 的 OptGroup 渲染
  const groupedOptions = useMemo(() => {
    const groups: Record<string, { label: string; value: string }[]> = {}
    for (const opt of allTypeOptions) {
      if (!groups[opt.group]) groups[opt.group] = []
      groups[opt.group].push({ label: opt.label, value: opt.value })
    }
    return groups
  }, [allTypeOptions])

  // 当前类型是否为非内置（rclone 动态后端）
  const isDynamicType = !isBuiltinType(draft.type)
  const staticFields = isBuiltinType(draft.type) ? getStorageTargetFieldConfigs(draft.type) : []

  // 当前 rclone 后端的动态字段
  const dynamicBackend = useMemo(() => {
    if (!isDynamicType) return null
    return rcloneBackends.find((b) => b.name === draft.type) || null
  }, [isDynamicType, draft.type, rcloneBackends])

  function updateConfig(key: string, value: string | boolean) {
    setDraft((c) => ({ ...c, config: { ...c.config, [key]: value } }))
  }

  function validate(value: StorageTargetPayload) {
    if (!value.name.trim()) return '请输入存储目标名称'
    if (!value.type.trim()) return '请选择存储类型'
    if (isBuiltinType(value.type)) {
      for (const field of staticFields) {
        if (!field.required || field.type === 'switch') continue
        const v = value.config[field.key]
        if (typeof v !== 'string' || !v.trim()) return `请填写${field.label}`
      }
    }
    return ''
  }

  async function handleSubmit() {
    const e = validate(draft); if (e) { setError(e); return }
    setError(''); await onSubmit(draft, initialValue?.id)
  }
  async function handleTest() {
    const e = validate(draft); if (e) { setError(e); return }
    setError(''); setTestResult(await onTest(draft, initialValue?.id))
  }
  async function handleGoogleDriveAuth() {
    const e = validate(draft); if (e) { setError(e); return }
    setError(''); await onGoogleDriveAuth(draft, initialValue?.id)
  }

  // 渲染静态字段（内置类型）
  function renderStaticFields() {
    return staticFields.map((field) => {
      const value = draft.config[field.key]
      const normalized = typeof value === 'boolean' ? value : typeof value === 'string' ? value : field.type === 'switch' ? false : ''
      return (
        <div key={field.key}>
          <Typography.Text>{field.label}{field.required ? ' *' : ''}</Typography.Text>
          {field.type === 'switch' ? (
            <Space align="center" size="medium">
              <Switch checked={Boolean(normalized)} onChange={(v) => updateConfig(field.key, v)} />
              {field.description && <Typography.Text type="secondary">{field.description}</Typography.Text>}
            </Space>
          ) : field.type === 'password' ? (
            <Input.Password value={String(normalized)} placeholder={field.placeholder} onChange={(v) => updateConfig(field.key, v)} />
          ) : (
            <Input value={String(normalized)} placeholder={field.placeholder} onChange={(v) => updateConfig(field.key, v)} />
          )}
          {field.description && field.type !== 'switch' && (
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>{field.description}</Typography.Paragraph>
          )}
          {initialValue?.maskedFields?.includes(field.key) && !draft.config[field.key] && (
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>已存在敏感配置，留空则保持不变。</Typography.Paragraph>
          )}
        </div>
      )
    })
  }

  // 渲染单个动态字段
  function renderDynamicOption(opt: RcloneBackendOption) {
    return (
      <div key={opt.key}>
        <Typography.Text>{opt.key}{opt.required ? ' *' : ''}</Typography.Text>
        {opt.isPassword ? (
          <Input.Password value={(draft.config[opt.key] as string) || ''} placeholder={opt.label} onChange={(v) => updateConfig(opt.key, v)} />
        ) : (
          <Input value={(draft.config[opt.key] as string) || ''} placeholder={opt.label} onChange={(v) => updateConfig(opt.key, v)} />
        )}
        {opt.label && (
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 2, fontSize: 12 }} ellipsis={{ rows: 2, expandable: true }}>{opt.label}</Typography.Paragraph>
        )}
      </div>
    )
  }

  // 渲染动态字段（rclone 后端）— 必填优先，可选折叠
  function renderDynamicFields() {
    const requiredOptions = dynamicBackend?.options.filter((opt) => opt.required) ?? []
    const optionalOptions = dynamicBackend?.options.filter((opt) => !opt.required) ?? []

    return (
      <>
        <div>
          <Typography.Text>远端路径</Typography.Text>
          <Input value={(draft.config.root as string) || ''} placeholder="如 /backups 或 bucket 名" onChange={(v) => updateConfig('root', v)} />
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>远端根路径、桶名或挂载点，留空使用根目录</Typography.Paragraph>
        </div>
        {requiredOptions.map(renderDynamicOption)}
        {optionalOptions.length > 0 && (
          <Collapse bordered={false} style={{ background: 'transparent' }}>
            <Collapse.Item
              header={<Typography.Text type="secondary">高级配置（{optionalOptions.length} 个可选项）</Typography.Text>}
              name="advanced"
            >
              <Space direction="vertical" size="large" style={{ width: '100%' }}>
                {optionalOptions.map(renderDynamicOption)}
              </Space>
            </Collapse.Item>
          </Collapse>
        )}
      </>
    )
  }

  return (
    <Drawer width={560} title={initialValue ? '编辑存储目标' : '新建存储目标'} visible={visible} onCancel={onCancel} unmountOnExit={false}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {error ? <Alert type="error" content={error} /> : <Alert type="info" content="存储目标提供备份文件的最终去向，请确保服务端网络连通性并通过测试。" />}
        {testResult && <Alert type={testResult.success ? 'success' : 'warning'} content={testResult.message} />}

        <div>
          <Typography.Text>名称</Typography.Text>
          <Input value={draft.name} placeholder="例如：生产环境 MinIO" onChange={(v) => setDraft((c) => ({ ...c, name: v }))} />
        </div>

        <div>
          <Typography.Text>存储类型</Typography.Text>
          <Select
            showSearch
            value={draft.type || undefined}
            placeholder="搜索存储类型..."
            filterOption={(input, option) => {
              const label = String(option?.props?.children ?? option?.props?.label ?? '')
              return label.toLowerCase().includes(input.toLowerCase())
            }}
            onChange={(value) => {
              setDraft((c) => ({ ...c, type: value as string, config: {} }))
              setTestResult(null)
            }}
          >
            {Object.entries(groupedOptions).map(([group, options]) => (
              <Select.OptGroup key={group} label={group}>
                {options.map((opt) => (
                  <Select.Option key={opt.value} value={opt.value}>{opt.label}</Select.Option>
                ))}
              </Select.OptGroup>
            ))}
          </Select>
        </div>

        <div>
          <Typography.Text>描述</Typography.Text>
          <Input.TextArea value={draft.description} placeholder="可选描述" onChange={(v) => setDraft((c) => ({ ...c, description: v }))} />
        </div>
        <div>
          <Typography.Text>容量配额（GB，0 = 不限制）</Typography.Text>
          <InputNumber
            style={{ width: '100%' }}
            min={0}
            value={Math.round((draft.quotaBytes ?? 0) / (1024 * 1024 * 1024))}
            onChange={(v) => {
              const gb = Number(v ?? 0)
              setDraft((c) => ({ ...c, quotaBytes: gb > 0 ? gb * 1024 * 1024 * 1024 : 0 }))
            }}
          />
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
            软配额：累计备份字节超出后拒绝新上传。与 85% 容量预警互补，防止失控。
          </Typography.Paragraph>
        </div>

        <Space align="center" size="medium">
          <Typography.Text>启用</Typography.Text>
          <Switch checked={draft.enabled} onChange={(v) => setDraft((c) => ({ ...c, enabled: v }))} />
        </Space>

        <Divider orientation="left">环境配置</Divider>

        <div>
          <Typography.Title heading={6} style={{ marginTop: 0, color: 'var(--color-text-2)' }}>
            {getStorageTargetTypeLabel(draft.type)}
          </Typography.Title>
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            {isDynamicType ? renderDynamicFields() : renderStaticFields()}
          </Space>
        </div>

        <Space>
          <Button loading={testing} onClick={handleTest}>测试连接</Button>
          {draft.type === 'google_drive' && (
            <Button type="outline" onClick={handleGoogleDriveAuth}>
              {initialValue ? '重新授权 Google Drive' : '发起 Google Drive 授权'}
            </Button>
          )}
          <Button type="primary" loading={loading} onClick={handleSubmit}>保存</Button>
        </Space>
      </Space>
    </Drawer>
  )
}
